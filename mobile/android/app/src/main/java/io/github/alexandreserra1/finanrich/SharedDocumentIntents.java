package io.github.alexandreserra1.finanrich;

import android.content.ClipData;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.pm.ProviderInfo;
import android.net.Uri;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Locale;
import java.util.Set;

final class SharedDocumentIntents {
    private static final Set<String> MIME_TYPES = new HashSet<>(Arrays.asList(
        "application/pdf", "text/csv", "application/csv", "application/x-csv",
        "application/ofx", "application/x-ofx", "application/vnd.intu.qfx",
        "application/qfx", "application/x-qfx",
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "image/png", "image/jpeg", "image/webp", "application/octet-stream"
    ));

    static String mimeType(Intent intent) {
        String type = intent.getType();
        if (type == null) return null;
        type = type.toLowerCase(Locale.ROOT);
        return MIME_TYPES.contains(type) ? type : null;
    }

    static Uri extract(Intent intent) {
        if (!Intent.ACTION_SEND.equals(intent.getAction()) || mimeType(intent) == null) return null;
        Object stream = intent.getExtras() == null ? null : intent.getExtras().get(Intent.EXTRA_STREAM);
        if (stream != null && !(stream instanceof Uri)) return null;
        Uri uri = (Uri) stream;
        ClipData clip = intent.getClipData();
        if (clip != null) {
            if (clip.getItemCount() != 1) return null;
            ClipData.Item item = clip.getItemAt(0);
            if (item.getIntent() != null || item.getUri() == null) return null;
            if (uri != null && !uri.equals(item.getUri())) return null;
            uri = item.getUri();
        }
        if (intent.getData() != null && !intent.getData().equals(uri)) return null;
        return validUri(uri) ? uri : null;
    }

    static boolean validUri(Uri uri) {
        if (uri == null || !"content".equals(uri.getScheme()) || !uri.isHierarchical()) return false;
        String authority = uri.getEncodedAuthority();
        return authority != null && authority.length() <= 255
            && authority.matches("[A-Za-z0-9_][A-Za-z0-9_.-]*")
            && uri.getFragment() == null;
    }

    static boolean externalProvider(Uri uri, PackageManager packages, int ownUid) {
        if (!validUri(uri)) return false;
        ProviderInfo provider = packages.resolveContentProvider(uri.getAuthority(), 0);
        return provider != null && provider.applicationInfo != null && provider.applicationInfo.uid != ownUid;
    }
}
