package io.github.alexandreserra1.finanrich;

import android.app.Notification;
import android.os.Bundle;
import android.service.notification.NotificationListenerService;
import android.service.notification.StatusBarNotification;
import java.util.Arrays;
import java.util.HashSet;
import java.util.Set;

/**
 * Guarda as notificações dos apps de banco (compra aprovada, Pix recebido...)
 * para o app transformar em lançamentos a revisar. Só os apps da lista; o
 * resto é ignorado sem ler. A pessoa liga em Ajustes do Android.
 */
public class BankNotificationService extends NotificationListenerService {
    static final Set<String> BANKS = new HashSet<>(Arrays.asList(
        "com.nu.production", // Nubank
        "com.itau", // Itaú
        "br.com.intermedium", // Inter
        "com.c6bank.app", // C6
        "br.com.bb.android", // Banco do Brasil
        "com.bradesco", // Bradesco
        "com.santander.app", // Santander
        "br.com.gabba.Caixa", // Caixa
        "br.gov.caixa.tem", // Caixa Tem
        "com.mercadopago.wallet", // Mercado Pago
        "com.picpay", // PicPay
        "br.com.neon", // Neon
        "br.com.uol.ps.myaccount" // PagBank
    ));

    @Override
    public void onNotificationPosted(StatusBarNotification sbn) {
        if (!BANKS.contains(sbn.getPackageName())) return;
        Bundle extras = sbn.getNotification().extras;
        CharSequence title = extras.getCharSequence(Notification.EXTRA_TITLE);
        CharSequence text = extras.getCharSequence(Notification.EXTRA_BIG_TEXT);
        if (text == null) text = extras.getCharSequence(Notification.EXTRA_TEXT);
        if (text == null) return;
        String full = (title != null ? title + ". " : "") + text;
        BankNotificationStore.add(this, sbn.getPackageName() + "|" + sbn.getPostTime(), sbn.getPostTime(), full);
    }
}
