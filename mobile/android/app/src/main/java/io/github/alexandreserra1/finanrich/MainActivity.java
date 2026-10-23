package io.github.alexandreserra1.finanrich;

import android.os.Bundle;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(BankNotificationsPlugin.class); // plugin do próprio app: registra antes de abrir
        super.onCreate(savedInstanceState);
    }
}
