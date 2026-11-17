package io.github.alexandreserra1.finanrich;

import android.content.Intent;
import android.database.Cursor;
import android.net.Uri;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.ParcelFileDescriptor;
import android.os.Process;
import android.os.SystemClock;
import android.provider.OpenableColumns;
import android.util.Base64;
import android.widget.Toast;
import com.getcapacitor.JSObject;
import com.getcapacitor.Plugin;
import com.getcapacitor.PluginCall;
import com.getcapacitor.PluginMethod;
import com.getcapacitor.annotation.CapacitorPlugin;
import java.io.IOException;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.ScheduledThreadPoolExecutor;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;

@CapacitorPlugin(name = "SharedDocuments")
public class SharedDocumentsPlugin extends Plugin {
    private final SharedDocumentStore documents = new SharedDocumentStore(SystemClock::elapsedRealtime);
    private final Handler main = new Handler(Looper.getMainLooper());
    private final ThreadPoolExecutor worker = new ThreadPoolExecutor(
        1, 1, 0, TimeUnit.MILLISECONDS, new ArrayBlockingQueue<>(1)
    );
    private final ScheduledThreadPoolExecutor timer = new ScheduledThreadPoolExecutor(1);
    private final ThreadPoolExecutor cleanup = new ThreadPoolExecutor(
        1, 1, 0, TimeUnit.MILLISECONDS, new ArrayBlockingQueue<>(1)
    );
    private volatile boolean destroyed;
    private Runnable expiry;

    @Override
    public void load() {
        timer.setRemoveOnCancelPolicy(true);
        receive(getActivity().getIntent());
    }

    @Override
    protected void handleOnNewIntent(Intent intent) {
        receive(intent);
    }

    private void receive(Intent intent) {
        if (destroyed || intent == null) return;
        String action = intent.getAction();
        if (!Intent.ACTION_SEND.equals(action) && !Intent.ACTION_SEND_MULTIPLE.equals(action)) return;
        try {
            Uri uri = SharedDocumentIntents.extract(intent);
            String mimeType = SharedDocumentIntents.mimeType(intent);
            if (uri == null || !SharedDocumentIntents.externalProvider(uri, getContext().getPackageManager(), Process.myUid())) {
                warn("Compartilhe apenas um arquivo PDF, CSV, OFX, QFX, XLSX ou uma imagem compatível.");
                return;
            }
            synchronized (documents) {
                SharedDocumentStore.Document document = documents.offer(uri.toString(), mimeType);
                if (document == null) {
                    warn("Há um arquivo aguardando ou em leitura. Compartilhe o próximo depois.");
                    return;
                }
                if (expiry != null) main.removeCallbacks(expiry);
                String id = document.id;
                expiry = () -> {
                    synchronized (documents) {
                        if (documents.discard(id)) changed();
                    }
                };
                main.postDelayed(expiry, SharedDocumentStore.TTL_MILLIS);
                changed();
                SharedDocumentAccess access = new SharedDocumentAccess(timer, cleanup, () -> {
                    synchronized (documents) {
                        documents.discard(id);
                        changed();
                    }
                    warn("O app de origem demorou para responder. Compartilhe o arquivo novamente depois.");
                });
                try {
                    worker.execute(() -> name(document, access));
                } catch (RejectedExecutionException error) {
                    access.close();
                    documents.discard(document.id);
                    changed();
                    warn("Não foi possível receber o arquivo. Compartilhe novamente depois.");
                }
            }
        } catch (SecurityException error) {
            warn("Não foi possível acessar o arquivo compartilhado. Compartilhe novamente pelo app de origem.");
        } catch (RuntimeException error) {
            warn("Não foi possível receber o arquivo. Compartilhe novamente pelo app de origem.");
        } finally {
            intent.replaceExtras((Bundle) null);
            intent.setClipData(null);
            intent.setDataAndType(null, null);
            intent.setAction(null);
        }
    }

    private void name(SharedDocumentStore.Document document, SharedDocumentAccess access) {
        Uri uri = Uri.parse(document.uri);
        try {
            if (destroyed || !SharedDocumentIntents.externalProvider(uri, getContext().getPackageManager(), Process.myUid())) return;
            try (Cursor cursor = getContext().getContentResolver().query(
                uri, new String[] { OpenableColumns.DISPLAY_NAME }, null, null, null, access.signal
            )) {
                if (cursor == null || !cursor.moveToFirst()) return;
                int column = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                if (column < 0) return;
                String name = cursor.getString(column);
                if (name == null || name.isEmpty()) return;
                name = name.replaceAll("[\\p{Cntrl}/\\\\]", "_");
                if (name.length() > 255) name = name.substring(0, 255);
                synchronized (documents) {
                    if (documents.rename(document.id, name)) changed();
                }
            }
        } catch (RuntimeException error) {
            warn("Não foi possível consultar o nome do arquivo. Você ainda pode tentar abri-lo.");
        } finally {
            access.close();
        }
    }

    @PluginMethod
    public void peek(PluginCall call) {
        synchronized (documents) {
            SharedDocumentStore.Document document = documents.peek();
            call.resolve(document == null ? new JSObject() : metadata(document));
        }
    }

    @PluginMethod
    public void read(PluginCall call) {
        synchronized (documents) {
            if (destroyed) {
                call.reject("O arquivo não está mais disponível. Compartilhe novamente.");
                return;
            }
            SharedDocumentStore.Document document = documents.begin(call.getString("id"));
            if (document == null) {
                call.reject("O arquivo não está mais disponível. Compartilhe novamente.");
                return;
            }
            changed();
            SharedDocumentAccess access = new SharedDocumentAccess(timer, cleanup, () -> {
                synchronized (documents) {
                    documents.discard(document.id);
                    call.reject("O app de origem demorou para responder. Compartilhe o arquivo novamente depois.");
                    changed();
                }
            });
            try {
                worker.execute(() -> readDocument(call, document, access));
            } catch (RejectedExecutionException error) {
                access.close();
                documents.finish(document.id);
                call.reject("Não foi possível ler o arquivo agora. Compartilhe novamente depois.");
            }
        }
    }

    private void readDocument(PluginCall call, SharedDocumentStore.Document document, SharedDocumentAccess access) {
        try {
            Uri uri = Uri.parse(document.uri);
            if (!documents.isActive(document.id) || destroyed) throw new IOException();
            if (!SharedDocumentIntents.externalProvider(uri, getContext().getPackageManager(), Process.myUid())) {
                throw new SecurityException();
            }
            ParcelFileDescriptor descriptor = getContext().getContentResolver().openFileDescriptor(uri, "r", access.signal);
            access.attach(descriptor);
            byte[] bytes = SharedDocumentStore.readLimited(
                new ParcelFileDescriptor.AutoCloseInputStream(descriptor),
                () -> !destroyed && documents.isActive(document.id)
            );
            JSObject result = metadata(document);
            result.put("base64", Base64.encodeToString(bytes, Base64.NO_WRAP));
            synchronized (documents) {
                if (!access.complete()) return;
                if (!documents.finish(document.id) || destroyed) {
                    call.reject("O arquivo expirou ou a leitura foi cancelada. Compartilhe novamente.");
                    return;
                }
                call.resolve(result);
            }
        } catch (SecurityException error) {
            if (access.complete()) call.reject("Sem acesso ao arquivo compartilhado. Compartilhe novamente pelo app de origem.");
        } catch (IOException error) {
            if (access.complete()) call.reject("O arquivo não pôde ser lido ou ultrapassa o limite de 8 MiB. Compartilhe novamente.");
        } catch (RuntimeException error) {
            if (access.complete()) call.reject("Não foi possível ler o arquivo compartilhado. Compartilhe novamente.");
        } finally {
            access.close();
            synchronized (documents) {
                documents.finish(document.id);
                changed();
            }
        }
    }

    @PluginMethod
    public void discard(PluginCall call) {
        synchronized (documents) {
            if (documents.discard(call.getString("id"))) changed();
            call.resolve();
        }
    }

    private JSObject metadata(SharedDocumentStore.Document document) {
        JSObject result = new JSObject();
        result.put("id", document.id);
        result.put("name", document.name);
        result.put("mimeType", document.mimeType);
        return result;
    }

    private void changed() {
        main.post(() -> {
            if (!destroyed) notifyListeners("pending", new JSObject());
        });
    }

    private void warn(String message) {
        main.post(() -> {
            if (!destroyed) Toast.makeText(getContext(), message, Toast.LENGTH_LONG).show();
        });
    }

    @Override
    protected void handleOnDestroy() {
        synchronized (documents) {
            destroyed = true;
            documents.clear();
            main.removeCallbacksAndMessages(null);
            worker.shutdownNow();
            timer.shutdown();
            cleanup.shutdown();
        }
    }
}
