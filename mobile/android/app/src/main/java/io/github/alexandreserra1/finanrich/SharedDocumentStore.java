package io.github.alexandreserra1.finanrich;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.util.UUID;
import java.util.function.BooleanSupplier;
import java.util.function.LongSupplier;

final class SharedDocumentStore {
    static final int MAX_BYTES = 8 * 1024 * 1024;
    static final long TTL_MILLIS = 10 * 60 * 1000;

    static final class Document {
        final String id;
        final String uri;
        final String mimeType;
        final long receivedAt;
        String name = "Documento compartilhado";

        Document(String uri, String mimeType, long receivedAt) {
            this.id = UUID.randomUUID().toString();
            this.uri = uri;
            this.mimeType = mimeType;
            this.receivedAt = receivedAt;
        }
    }

    private final LongSupplier clock;
    private Document pending;
    private Document active;
    private boolean cancelled;

    SharedDocumentStore(LongSupplier clock) {
        this.clock = clock;
    }

    synchronized Document offer(String uri, String mimeType) {
        if (peek() != null || active != null) return null;
        pending = new Document(uri, mimeType, clock.getAsLong());
        return pending;
    }

    synchronized Document peek() {
        if (pending != null && expired(pending)) pending = null;
        if (pending != null) return pending;
        return active != null && isActive(active.id) ? active : null;
    }

    synchronized boolean rename(String id, String name) {
        Document document = peek();
        if (document == null || !document.id.equals(id)) return false;
        document.name = name;
        return true;
    }

    synchronized Document begin(String id) {
        Document document = peek();
        if (document == null || !document.id.equals(id) || active != null) return null;
        active = document;
        pending = null;
        cancelled = false;
        return document;
    }

    synchronized boolean discard(String id) {
        if (pending != null && pending.id.equals(id)) {
            pending = null;
            return true;
        }
        if (active != null && active.id.equals(id)) {
            cancelled = true;
            return true;
        }
        return false;
    }

    synchronized boolean isActive(String id) {
        return active != null && active.id.equals(id) && !cancelled && !expired(active);
    }

    synchronized boolean finish(String id) {
        if (active == null || !active.id.equals(id)) return false;
        boolean valid = isActive(id);
        active = null;
        cancelled = false;
        return valid;
    }

    synchronized void clear() {
        pending = null;
        cancelled = true;
    }

    private boolean expired(Document document) {
        return clock.getAsLong() - document.receivedAt >= TTL_MILLIS;
    }

    static byte[] readLimited(InputStream input, BooleanSupplier valid) throws IOException {
        if (input == null) throw new IOException("Não foi possível abrir o arquivo compartilhado.");
        try (InputStream stream = input; ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            byte[] buffer = new byte[8192];
            while (true) {
                if (Thread.currentThread().isInterrupted() || !valid.getAsBoolean()) {
                    throw new IOException("O arquivo expirou ou a leitura foi cancelada. Compartilhe novamente.");
                }
                int count = stream.read(buffer, 0, Math.min(buffer.length, MAX_BYTES - output.size() + 1));
                if (count == -1) break;
                if (count == 0) {
                    int single = stream.read();
                    if (single == -1) break;
                    buffer[0] = (byte) single;
                    count = 1;
                }
                if (count > MAX_BYTES - output.size()) {
                    throw new IOException("O arquivo ultrapassa o limite de 8 MiB.");
                }
                output.write(buffer, 0, count);
            }
            if (!valid.getAsBoolean()) throw new IOException("O arquivo expirou ou a leitura foi cancelada. Compartilhe novamente.");
            return output.toByteArray();
        }
    }
}
