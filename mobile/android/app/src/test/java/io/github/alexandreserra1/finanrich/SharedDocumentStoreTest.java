package io.github.alexandreserra1.finanrich;

import static org.junit.Assert.*;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.Test;

public class SharedDocumentStoreTest {
    private long now;
    private final SharedDocumentStore store = new SharedDocumentStore(() -> now);

    private SharedDocumentStore.Document offer() {
        return store.offer("content://banco/documento", "application/pdf");
    }

    @Test
    public void chegadaNaoSubstituiPendencia() {
        SharedDocumentStore.Document first = offer();
        assertNull(offer());
        assertEquals(first.id, store.peek().id);
        assertFalse(store.discard("outro"));
        assertEquals(first.id, store.peek().id);
    }

    @Test
    public void expiraEmDezMinutosSemRenovarAoConsultar() {
        offer();
        now = SharedDocumentStore.TTL_MILLIS - 1;
        assertNotNull(store.peek());
        now++;
        assertNull(store.peek());
        assertNotNull(offer());
    }

    @Test
    public void leituraConsomeTokenUmaVezEBloqueiaChegadaAteTerminar() {
        SharedDocumentStore.Document first = offer();
        assertNull(store.begin("outro"));
        assertNotNull(store.begin(first.id));
        assertEquals(first.id, store.peek().id);
        assertNull(store.begin(first.id));
        assertNull(offer());
        assertTrue(store.finish(first.id));
        SharedDocumentStore.Document next = offer();
        assertNotEquals(first.id, next.id);
        assertFalse(store.discard(first.id));
        assertFalse(store.finish(first.id));
        assertEquals(next.id, store.peek().id);
    }

    @Test
    public void descarteCancelaLeituraSemLiberarOutraConcorrente() {
        SharedDocumentStore.Document first = offer();
        store.begin(first.id);
        assertTrue(store.discard(first.id));
        assertNull(store.peek());
        assertFalse(store.isActive(first.id));
        assertNull(offer());
        assertFalse(store.finish(first.id));
        assertNotNull(offer());
    }

    @Test
    public void leituraExpiradaNaoEntregaConteudo() {
        SharedDocumentStore.Document first = offer();
        store.begin(first.id);
        now = SharedDocumentStore.TTL_MILLIS;
        assertFalse(store.isActive(first.id));
        assertFalse(store.finish(first.id));
    }

    @Test
    public void apenasUmaChamadaConcorrenteConsomeToken() throws Exception {
        String id = offer().id;
        CountDownLatch start = new CountDownLatch(1);
        AtomicInteger claimed = new AtomicInteger();
        Runnable read = () -> {
            try {
                start.await();
                if (store.begin(id) != null) claimed.incrementAndGet();
            } catch (InterruptedException error) {
                Thread.currentThread().interrupt();
            }
        };
        Thread first = new Thread(read);
        Thread second = new Thread(read);
        first.start();
        second.start();
        start.countDown();
        first.join();
        second.join();
        assertEquals(1, claimed.get());
    }

    @Test
    public void aceitaOitoMiBExatos() throws Exception {
        byte[] bytes = new byte[SharedDocumentStore.MAX_BYTES];
        bytes[bytes.length - 1] = 42;
        assertArrayEquals(bytes, SharedDocumentStore.readLimited(new ByteArrayInputStream(bytes), () -> true));
    }

    @Test
    public void limitaStreamMesmoSemTamanhoDeclaradoEFechaNaFalha() {
        AtomicInteger read = new AtomicInteger();
        AtomicInteger closed = new AtomicInteger();
        InputStream endless = new InputStream() {
            @Override public int read() { read.incrementAndGet(); return 1; }
            @Override public int read(byte[] buffer, int offset, int length) {
                read.addAndGet(length);
                return length;
            }
            @Override public void close() { closed.incrementAndGet(); }
        };
        IOException error = assertThrows(IOException.class, () -> SharedDocumentStore.readLimited(endless, () -> true));
        assertEquals("O arquivo ultrapassa o limite de 8 MiB.", error.getMessage());
        assertEquals(SharedDocumentStore.MAX_BYTES + 1, read.get());
        assertEquals(1, closed.get());
    }

    @Test
    public void cancelamentoNaoLeStream() {
        InputStream forbidden = new InputStream() {
            @Override public int read() { fail("Não deve ler após cancelar."); return -1; }
        };
        assertThrows(IOException.class, () -> SharedDocumentStore.readLimited(forbidden, () -> false));
    }

    @Test
    public void erroDoProvedorNaoViraLeituraParcial() {
        InputStream broken = new InputStream() {
            @Override public int read() throws IOException { throw new IOException("falha"); }
        };
        assertThrows(IOException.class, () -> SharedDocumentStore.readLimited(broken, () -> true));
    }

    @Test
    public void metadadosAtrasadosNaoAlteramOutroToken() {
        String old = offer().id;
        assertTrue(store.discard(old));
        String current = offer().id;
        assertFalse(store.rename(old, "antigo.pdf"));
        assertEquals(current, store.peek().id);
        assertEquals("Documento compartilhado", store.peek().name);
        assertTrue(store.rename(current, "extrato.pdf"));
        assertEquals("extrato.pdf", store.peek().name);
    }

    @Test
    public void nomeConsultadoAntesDaLeituraFicaNoMesmoDocumento() {
        SharedDocumentStore.Document document = offer();
        store.begin(document.id);
        assertTrue(store.rename(document.id, "extrato.ofx"));
        assertEquals("extrato.ofx", document.name);
        store.discard(document.id);
        assertFalse(store.rename(document.id, "outro.ofx"));
    }

    @Test
    public void destruicaoDescartaPendenciaECancelaLeitura() {
        SharedDocumentStore.Document document = offer();
        store.begin(document.id);
        store.clear();
        assertNull(store.peek());
        assertFalse(store.isActive(document.id));
        assertFalse(store.finish(document.id));
    }

    @Test
    public void streamComLeiturasCurtasEVaziasNaoPerdeBytes() throws Exception {
        InputStream stream = new ByteArrayInputStream(new byte[] { 10, 20, 30 }) {
            private boolean empty = true;
            @Override public synchronized int read(byte[] buffer, int offset, int length) {
                if (empty) {
                    empty = false;
                    return 0;
                }
                return super.read(buffer, offset, Math.min(length, 1));
            }
        };
        assertArrayEquals(new byte[] { 10, 20, 30 }, SharedDocumentStore.readLimited(stream, () -> true));
    }

    @Test
    public void cancelamentoDuranteStreamNaoEntregaLeituraParcial() {
        AtomicInteger read = new AtomicInteger();
        InputStream stream = new InputStream() {
            @Override public int read() { read.incrementAndGet(); return 42; }
            @Override public int read(byte[] buffer, int offset, int length) { return read(); }
        };
        assertThrows(IOException.class, () -> SharedDocumentStore.readLimited(stream, () -> read.get() == 0));
    }

    @Test
    public void provedorQueNaoAbreStreamFalhaComMensagemClara() {
        IOException error = assertThrows(IOException.class, () -> SharedDocumentStore.readLimited(null, () -> true));
        assertEquals("Não foi possível abrir o arquivo compartilhado.", error.getMessage());
    }

    @Test
    public void conclusaoAtrasadaNaoCancelaNovaLeitura() {
        String old = offer().id;
        store.begin(old);
        store.finish(old);
        String current = offer().id;
        store.begin(current);
        assertFalse(store.finish(old));
        assertFalse(store.discard(old));
        assertTrue(store.isActive(current));
    }

    @Test
    public void fechaStreamTambemNoSucesso() throws Exception {
        AtomicInteger closed = new AtomicInteger();
        InputStream stream = new ByteArrayInputStream(new byte[] { 42 }) {
            @Override public void close() { closed.incrementAndGet(); }
        };
        assertArrayEquals(new byte[] { 42 }, SharedDocumentStore.readLimited(stream, () -> true));
        assertEquals(1, closed.get());
    }
}
