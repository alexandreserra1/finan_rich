package io.github.alexandreserra1.finanrich;

import android.content.Context;
import android.content.SharedPreferences;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

/**
 * Fila das notificações de banco, nas preferências privadas do app, até o app
 * abrir e ler. Ler apaga: o texto não fica guardado depois.
 */
final class BankNotificationStore {
    private static final String PREFS = "bank-notifications";
    private static final String KEY = "queue";
    private static final int MAX = 300; // celular sem abrir o app por meses não enche a memória

    private BankNotificationStore() {}

    static synchronized void add(Context ctx, String id, long time, String text) {
        SharedPreferences prefs = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        try {
            JSONArray queue = new JSONArray(prefs.getString(KEY, "[]"));
            for (int i = 0; i < queue.length(); i++) {
                if (id.equals(queue.getJSONObject(i).optString("id"))) return; // a mesma notificação atualizada
            }
            JSONObject item = new JSONObject();
            item.put("id", id);
            item.put("time", time);
            item.put("text", text.length() > 500 ? text.substring(0, 500) : text);
            queue.put(item);
            while (queue.length() > MAX) queue.remove(0);
            prefs.edit().putString(KEY, queue.toString()).apply();
        } catch (JSONException e) {
            prefs.edit().remove(KEY).apply();
        }
    }

    static synchronized JSONArray drain(Context ctx) {
        SharedPreferences prefs = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        JSONArray queue;
        try {
            queue = new JSONArray(prefs.getString(KEY, "[]"));
        } catch (JSONException e) {
            queue = new JSONArray();
        }
        prefs.edit().remove(KEY).apply();
        return queue;
    }
}
