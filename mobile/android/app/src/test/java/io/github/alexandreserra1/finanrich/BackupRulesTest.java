package io.github.alexandreserra1.finanrich;

import static org.junit.Assert.*;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import javax.xml.parsers.DocumentBuilderFactory;
import org.junit.Test;
import org.w3c.dom.Document;
import org.w3c.dom.Element;
import org.w3c.dom.NodeList;

// O backup do Google só leva os dados da Rich quando é cifrado de ponta a ponta
// (decisão de 18/11/2026, docs/SEGURANCA.md). Estes testes travam as regras.
public class BackupRulesTest {
    private static Document xml(String path) throws Exception {
        DocumentBuilderFactory f = DocumentBuilderFactory.newInstance();
        f.setNamespaceAware(true);
        return f.newDocumentBuilder().parse(new File("src/main/res/" + path));
    }

    @Test
    public void manifestoUsaAsRegras() throws Exception {
        String manifest = new String(Files.readAllBytes(new File("src/main/AndroidManifest.xml").toPath()), StandardCharsets.UTF_8);
        assertTrue(manifest.contains("android:fullBackupContent=\"@xml/backup_rules\""));
        assertTrue(manifest.contains("android:dataExtractionRules=\"@xml/data_extraction_rules\""));
    }

    @Test
    public void android12SoCopiaComCifra() throws Exception {
        NodeList cloud = xml("xml/data_extraction_rules.xml").getElementsByTagName("cloud-backup");
        assertEquals(1, cloud.getLength());
        assertEquals("true", ((Element) cloud.item(0)).getAttribute("disableIfNoEncryptionCapabilities"));
    }

    @Test
    public void android9a11SoCopiaCifradoOuDiretoParaOutroCelular() throws Exception {
        NodeList include = xml("xml-v28/backup_rules.xml").getElementsByTagName("include");
        assertEquals(2, include.getLength());
        for (int i = 0; i < include.getLength(); i++) {
            String flags = ((Element) include.item(i)).getAttribute("requireFlags");
            assertTrue(flags, flags.equals("clientSideEncryption") || flags.equals("deviceToDeviceTransfer"));
        }
    }

    @Test
    public void android7e8NaoCopia() throws Exception {
        Document rules = xml("xml/backup_rules.xml");
        assertEquals(0, rules.getElementsByTagName("include").getLength());
        Element exclude = (Element) rules.getElementsByTagName("exclude").item(0);
        assertEquals("root", exclude.getAttribute("domain"));
        assertEquals(".", exclude.getAttribute("path"));
    }
}
