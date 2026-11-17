package io.github.alexandreserra1.finanrich;

import static org.junit.Assert.*;

import android.content.ClipData;
import android.content.Context;
import android.content.Intent;
import android.net.Uri;
import android.os.Process;
import androidx.test.platform.app.InstrumentationRegistry;
import java.util.ArrayList;
import org.junit.Test;

public class SharedDocumentIntentsTest {
    private Intent share(String uri) {
        return new Intent(Intent.ACTION_SEND).setType("application/pdf")
            .putExtra(Intent.EXTRA_STREAM, Uri.parse(uri));
    }

    @Test
    public void aceitaArquivoUnicoComExtraEClipCorrespondentes() {
        Uri uri = Uri.parse("content://banco.arquivos/documento/1");
        Intent intent = share(uri.toString());
        intent.setClipData(ClipData.newRawUri("arquivo", uri));
        assertEquals(uri, SharedDocumentIntents.extract(intent));
    }

    @Test
    public void aceitaArquivoUnicoSomenteNoClip() {
        Uri uri = Uri.parse("content://banco.arquivos/documento/1");
        Intent intent = new Intent(Intent.ACTION_SEND).setType("text/csv");
        intent.setClipData(ClipData.newRawUri("arquivo", uri));
        assertEquals(uri, SharedDocumentIntents.extract(intent));
    }

    @Test
    public void rejeitaEsquemasAutoridadesEFormatosInvalidos() {
        String[] invalid = {
            "file:///data/arquivo.pdf", "https://banco/arquivo.pdf", "content:arquivo",
            "content:///arquivo", "content://usuario@banco/arquivo", "content://banco:80/arquivo",
            "content://%62anco/arquivo", "content://banco/arquivo#outro",
            "intent://banco/arquivo", "content://banco%2foutro/arquivo"
        };
        for (String uri : invalid) assertNull(uri, SharedDocumentIntents.extract(share(uri)));
    }

    @Test
    public void rejeitaTextoSoltoEStreamQueNaoSejaUri() {
        assertNull(SharedDocumentIntents.extract(new Intent(Intent.ACTION_SEND)
            .setType("application/pdf").putExtra(Intent.EXTRA_TEXT, "https://banco/arquivo.pdf")));
        assertNull(SharedDocumentIntents.extract(new Intent(Intent.ACTION_SEND)
            .setType("application/pdf").putExtra(Intent.EXTRA_STREAM, "content://banco/arquivo")));
        assertNull(SharedDocumentIntents.extract(share("content://banco/arquivo").setType("text/plain")));
    }

    @Test
    public void rejeitaMultiplosArquivosEClipDivergente() {
        Intent intent = share("content://banco/1");
        assertNull(SharedDocumentIntents.extract(new Intent(intent).setAction(Intent.ACTION_SEND_MULTIPLE)));
        ArrayList<Uri> files = new ArrayList<>();
        files.add(Uri.parse("content://banco/1"));
        assertNull(SharedDocumentIntents.extract(new Intent(Intent.ACTION_SEND).setType("application/pdf")
            .putParcelableArrayListExtra(Intent.EXTRA_STREAM, files)));
        ClipData clip = ClipData.newRawUri("arquivo", Uri.parse("content://banco/1"));
        clip.addItem(new ClipData.Item(Uri.parse("content://banco/2")));
        Intent multiple = new Intent(intent);
        multiple.setClipData(clip);
        assertNull(SharedDocumentIntents.extract(multiple));
        Intent different = new Intent(intent);
        different.setClipData(ClipData.newRawUri("arquivo", Uri.parse("content://banco/2")));
        assertNull(SharedDocumentIntents.extract(different));
        assertNull(SharedDocumentIntents.extract(new Intent(intent).setDataAndType(Uri.parse("content://banco/2"), "application/pdf")));
    }

    @Test
    public void rejeitaClipComIntentENaoBuscaLinkEmTexto() {
        Intent intent = new Intent(Intent.ACTION_SEND).setType("application/pdf");
        intent.setClipData(ClipData.newIntent("arquivo", share("content://banco/1")));
        assertNull(SharedDocumentIntents.extract(intent));
        intent.setClipData(ClipData.newPlainText("arquivo", "content://banco/1"));
        assertNull(SharedDocumentIntents.extract(intent));
    }

    @Test
    public void rejeitaProviderDoProprioUidEAutoridadeInexistente() {
        Context context = InstrumentationRegistry.getInstrumentation().getTargetContext();
        Uri own = Uri.parse("content://" + context.getPackageName() + ".fileprovider/documento");
        assertNotNull(context.getPackageManager().resolveContentProvider(own.getAuthority(), 0));
        assertFalse(SharedDocumentIntents.externalProvider(own, context.getPackageManager(), Process.myUid()));
        assertFalse(SharedDocumentIntents.externalProvider(Uri.parse("content://provider.inexistente.rich/documento"),
            context.getPackageManager(), Process.myUid()));
    }
}
