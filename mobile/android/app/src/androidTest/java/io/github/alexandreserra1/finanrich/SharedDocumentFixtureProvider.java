package io.github.alexandreserra1.finanrich;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.Bundle;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import java.io.File;
import java.io.FileNotFoundException;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;

public class SharedDocumentFixtureProvider extends ContentProvider {
    private int opens;

    @Override
    public boolean onCreate() { return true; }

    @Override
    public String getType(Uri uri) { return "text/csv"; }

    @Override
    public Cursor query(Uri uri, String[] projection, String selection, String[] selectionArgs, String sortOrder) {
        MatrixCursor result = new MatrixCursor(new String[] { OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE });
        result.addRow(new Object[] { "extrato-ficticio.csv", 1 });
        return result;
    }

    @Override
    public synchronized Bundle call(String method, String argument, Bundle extras) {
        if ("reset".equals(method)) opens = 0;
        Bundle result = new Bundle();
        result.putInt("opens", opens);
        return result;
    }

    @Override
    public synchronized ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        if (!"r".equals(mode) || !("/extrato.csv".equals(uri.getPath()) || "/grande.csv".equals(uri.getPath()))) throw new FileNotFoundException("Arquivo de teste inexistente.");
        opens++;
        File file = new File(getContext().getCacheDir(), "fixture.csv");
        try (FileOutputStream stream = new FileOutputStream(file)) {
            if ("/grande.csv".equals(uri.getPath())) {
                byte[] block = new byte[8192];
                for (int index = 0; index < 1024; index++) stream.write(block);
                stream.write(1);
            } else {
                stream.write("Data;Descrição;Valor\n16/11/2026;Mercado do emulador;-45,90\n".getBytes(StandardCharsets.UTF_8));
            }
        } catch (IOException error) { throw new FileNotFoundException("Falha ao preparar dados fictícios."); }
        return ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY);
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) { throw new UnsupportedOperationException(); }
    @Override
    public int delete(Uri uri, String selection, String[] arguments) { throw new UnsupportedOperationException(); }
    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] arguments) { throw new UnsupportedOperationException(); }
}
