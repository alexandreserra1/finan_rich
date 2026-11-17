package io.github.alexandreserra1.finanrich;

import android.os.CancellationSignal;
import android.os.ParcelFileDescriptor;
import java.io.IOException;
import java.util.concurrent.Executor;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

final class SharedDocumentAccess implements AutoCloseable {
    static final long TIMEOUT_SECONDS = 30;
    final CancellationSignal signal = new CancellationSignal();
    private final AtomicBoolean completed = new AtomicBoolean();
    private final ScheduledFuture<?> deadline;
    private ParcelFileDescriptor descriptor;
    private boolean closed;

    SharedDocumentAccess(ScheduledExecutorService timer, Executor cleanup, Runnable timeout) {
        deadline = timer.schedule(() -> {
            if (completed.compareAndSet(false, true)) {
                try {
                    timeout.run();
                } finally {
                    try {
                        cleanup.execute(this::close);
                    } catch (RejectedExecutionException ignored) {
                    }
                }
            }
        }, TIMEOUT_SECONDS, TimeUnit.SECONDS);
    }

    synchronized void attach(ParcelFileDescriptor file) throws IOException {
        if (file == null) throw new IOException("Não foi possível abrir o arquivo compartilhado.");
        if (closed || completed.get()) {
            file.close();
            throw new IOException("A leitura do arquivo foi cancelada.");
        }
        descriptor = file;
    }

    boolean complete() {
        boolean first = completed.compareAndSet(false, true);
        deadline.cancel(false);
        return first;
    }

    @Override
    public void close() {
        completed.set(true);
        deadline.cancel(false);
        ParcelFileDescriptor file;
        synchronized (this) {
            if (closed) return;
            closed = true;
            file = descriptor;
            descriptor = null;
        }
        try {
            if (file != null) file.close();
        } catch (IOException ignored) {
        } finally {
            try {
                signal.cancel();
            } catch (RuntimeException ignored) {
            }
        }
    }
}
