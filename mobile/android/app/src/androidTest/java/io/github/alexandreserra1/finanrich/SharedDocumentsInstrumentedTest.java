package io.github.alexandreserra1.finanrich;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertTrue;

import android.content.ClipData;
import android.content.Context;
import android.content.Intent;
import android.graphics.Bitmap;
import android.net.Uri;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry;
import androidx.test.runner.lifecycle.Stage;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import java.io.File;
import java.io.FileOutputStream;
import org.junit.Test;
import org.junit.runner.RunWith;

@RunWith(AndroidJUnit4.class)
public class SharedDocumentsInstrumentedTest {
    private static final Uri SOURCE = Uri.parse("content://io.github.alexandreserra1.finanrich.test.documents/extrato.csv");

    private Intent share(Context context, Uri uri) {
        Intent intent = new Intent(Intent.ACTION_SEND).setClass(context, MainActivity.class).setType("text/csv")
            .putExtra(Intent.EXTRA_STREAM, uri)
            .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
        intent.setClipData(ClipData.newRawUri("Extrato fictício", uri));
        return intent;
    }

    private String script(MainActivity activity, String source) throws Exception {
        CountDownLatch finished = new CountDownLatch(1);
        AtomicReference<String> value = new AtomicReference<>();
        InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> activity.getBridge().getWebView().evaluateJavascript(source, result -> {
            value.set(result);
            finished.countDown();
        }));
        assertTrue("A ponte deve responder.", finished.await(5, TimeUnit.SECONDS));
        return value.get();
    }

    private void until(MainActivity scenario, String condition) throws Exception {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(45);
        while (System.nanoTime() < deadline) {
            if ("true".equals(script(scenario, condition))) return;
            Thread.sleep(100);
        }
        throw new AssertionError("A tela não chegou ao estado esperado: " + condition);
    }

    private int opens(Context context) {
        return context.getContentResolver().call(SOURCE, "count", null, null).getInt("opens");
    }

    private MainActivity launch(Context context) throws Exception {
        context.startActivity(share(context, SOURCE));
        AtomicReference<MainActivity> activity = new AtomicReference<>();
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(30);
        while (System.nanoTime() < deadline) {
            InstrumentationRegistry.getInstrumentation().runOnMainSync(() -> {
                for (android.app.Activity candidate : ActivityLifecycleMonitorRegistry.getInstance().getActivitiesInStage(Stage.RESUMED)) {
                    if (candidate instanceof MainActivity) activity.set((MainActivity) candidate);
                }
            });
            if (activity.get() != null) return activity.get();
            Thread.sleep(100);
        }
        throw new AssertionError("O app não abriu.");
    }

    @Test
    public void compartilhamentoFrioQuenteLimiteERevisaoExplicita() throws Exception {
        Context context = InstrumentationRegistry.getInstrumentation().getTargetContext();
        assertNull(SharedDocumentIntents.extract(share(context, Uri.parse("file:///data/private.csv"))));
        assertNull(SharedDocumentIntents.extract(share(context, Uri.parse("https://example.invalid/extrato.csv"))));
        assertNull(SharedDocumentIntents.extract(share(context, SOURCE).setType("image/svg+xml")));
        assertNull(SharedDocumentIntents.extract(share(context, SOURCE).setAction(Intent.ACTION_SEND_MULTIPLE)));
        Intent multiple = share(context, SOURCE);
        multiple.getClipData().addItem(new ClipData.Item(Uri.parse(SOURCE + "?outro")));
        assertNull(SharedDocumentIntents.extract(multiple));
        assertFalse(SharedDocumentIntents.externalProvider(Uri.parse("content://io.github.alexandreserra1.finanrich.fileprovider/private.csv"), context.getPackageManager(), android.os.Process.myUid()));
        context.getContentResolver().call(SOURCE, "reset", null, null);
        MainActivity scenario = launch(context);
        try {
            until(scenario, "!!document.querySelector('[data-shared-review]')");
            assertEquals("Receber e consultar metadados não abre o conteúdo.", 0, opens(context));
            script(scenario, "document.querySelector('[data-shared-review]').click()");
            until(scenario, "document.querySelector('[data-field=description]')?.value === 'Mercado do emulador'");
            assertEquals(1, opens(context));
            File screenshot = new File(context.getExternalFilesDir(null), "share-preview.png");
            Bitmap image = InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();
            assertTrue("A captura do emulador deve existir.", image != null);
            try (FileOutputStream output = new FileOutputStream(screenshot)) {
                assertTrue(image.compress(Bitmap.CompressFormat.PNG, 100, output));
            } finally { image.recycle(); }
            assertEquals("Nada é salvo automaticamente.", "0", script(scenario, "document.querySelectorAll('.status-importado').length"));
            script(scenario, "window.__shareDialogs=0; window.confirm=()=>{window.__shareDialogs++;return false;}; document.querySelector('[data-field=description]').value='Correção preservada'; document.querySelector('[data-field=description]').dispatchEvent(new Event('change',{bubbles:true}))");
            context.startActivity(share(context, SOURCE));
            until(scenario, "!!document.querySelector('[data-shared-review]')");
            assertEquals("\"Correção preservada\"", script(scenario, "document.querySelector('[data-field=description]').value"));
            script(scenario, "document.querySelector('[data-shared-review]').click()");
            assertEquals("1", script(scenario, "window.__shareDialogs"));
            assertEquals("Recusar a troca não lê outro arquivo.", 1, opens(context));
            script(scenario, "document.querySelector('[data-shared-discard]').click()");
            until(scenario, "!document.querySelector('[data-shared-review]')");
            context.startActivity(share(context, Uri.parse("content://io.github.alexandreserra1.finanrich.test.documents/grande.csv")));
            until(scenario, "!!document.querySelector('[data-shared-review]')");
            script(scenario, "window.confirm=()=>true; document.querySelector('[data-shared-review]').click()");
            until(scenario, "!!document.querySelector('input[name=statement]') && !document.querySelector('.import-table')");
            until(scenario, "document.getElementById('flash')?.textContent.includes('8 MiB')");
            assertEquals("O tamanho real é verificado mesmo quando o provedor anuncia um byte.", 2, opens(context));
            assertEquals("false", script(scenario, "document.documentElement.classList.contains('app-locked')"));
        } finally { InstrumentationRegistry.getInstrumentation().runOnMainSync(scenario::finish); }
    }
}
