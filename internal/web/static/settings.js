// Tela de configurações da IA: preenche os campos conforme o provedor escolhido.
(function () {
  const { presets, installed, saved } = window.FINAN;
  const form = document.getElementById("ai-form");
  const byId = Object.fromEntries(presets.map((p) => [p.ID, p]));
  const field = (name) => form.querySelector(`[data-field="${name}"]`);
  const datalist = document.getElementById("model-options");

  // Valores salvos, para restaurar quando o usuário voltar ao provedor atual.
  const savedValues = { Model: form.elements.model.value, BaseURL: form.elements.base_url.value };

  function apply(preset, fill) {
    if (fill) {
      const values = preset.ID === saved ? savedValues : preset;
      form.elements.model.value = values.Model;
      form.elements.base_url.value = values.BaseURL;
    }
    // Endereço só importa para Ollama (se mudou) e APIs compatíveis com OpenAI.
    // No grátis do finan_rich, endereço e modelo são do servidor.
    field("base_url").hidden = preset.Kind === "anthropic" || preset.ID === "finan";
    if (field("model")) field("model").hidden = preset.ID === "finan";
    field("api_key").hidden = !preset.NeedsKey && preset.ID !== "custom";
    const link = field("api_key").querySelector("[data-key-link]");
    link.innerHTML = preset.KeyURL ? `Gere sua chave em <a href="${preset.KeyURL}" target="_blank" rel="noopener">${new URL(preset.KeyURL).host}</a>` : "";
    const models = new Set([...(preset.Kind === "ollama" ? installed || [] : []), ...(preset.Models || [])]);
    datalist.innerHTML = [...models].map((m) => `<option value="${m}">`).join("");
  }

  form.querySelectorAll('input[name="preset"]').forEach((radio) =>
    radio.addEventListener("change", () => apply(byId[radio.value], true)));
  apply(byId[form.elements.preset.value], false);

  // Testar usa a configuração salva; avisa se há alterações não salvas.
  const result = form.querySelector(".test-result");
  let dirty = false;
  form.addEventListener("input", () => { dirty = true; });
  form.querySelector("[data-test]").addEventListener("click", async (e) => {
    if (dirty) { result.textContent = "Salve antes de testar."; return; }
    e.target.disabled = true;
    result.textContent = "Testando… (no Ollama, a primeira resposta pode demorar enquanto o modelo carrega)";
    try {
      const res = await fetch("/configuracoes/testar", { method: "POST" });
      result.textContent = await res.text();
    } catch (err) {
      result.textContent = "❌ " + err.message;
    } finally {
      e.target.disabled = false;
    }
  });
})();
