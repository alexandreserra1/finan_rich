package proxy

// Registro de tokens por pedido (plano B3): uma linha no log para cada
// resposta do provedor, para medir o custo por pessoa e por personagem. A
// Vercel guarda o log (aba Logs). Nunca vai para o log o texto da pessoa, a
// chave do provedor nem o id cru do aparelho.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type usageRecord struct {
	Provider string `json:"provedor"`
	Model    string `json:"modelo"`
	Status   int    `json:"status"` // 0: o provedor não respondeu
	In       int    `json:"entrada"`
	Out      int    `json:"saida"` // -1: o provedor não informou
	Ms       int64  `json:"ms"`
	Persona  string `json:"personagem"`
	Device   string `json:"aparelho"` // resumo (hash) do id; vazio sem id
	start    time.Time
}

func (r usageRecord) log(status, in, out int) {
	r.Status, r.In, r.Out, r.Ms = status, in, out, time.Since(r.start).Milliseconds()
	b, _ := json.Marshal(r)
	log.Printf("ia %s", b)
}

// hashDevice: dá para contar pedidos por aparelho, não dá para voltar ao id.
func hashDevice(device string) string {
	if device == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("finan_rich:" + device))
	return hex.EncodeToString(sum[:6])
}

// personaRe acha o personagem nas instruções ("Você é o Bento, ...", de
// internal/advisor/personas.go). Só letras e no máximo 20: quem copiar o
// prompt não escreve o que quiser no log.
var personaRe = regexp.MustCompile(`(?m)^Você é (?:o|a) (\p{Lu}\p{L}{1,19})`)

// personaOf diz quem do time respondeu: o nome do personagem, "consultor"
// (sem personagem), "categorize" (categorização de extrato) ou "ping".
func personaOf(msgs []chatMessage) string {
	if len(msgs) == 0 || msgs[0].Role != "system" {
		return "ping"
	}
	if strings.HasPrefix(msgs[0].Content, advisor.CategorizeMarker) {
		return "categorize"
	}
	if m := personaRe.FindStringSubmatch(msgs[0].Content); m != nil {
		return strings.NewReplacer("á", "a").Replace(strings.ToLower(m[1])) // o mesmo id de finance.Team ("otavio")
	}
	return "consultor"
}

// tailBuffer guarda só o fim da resposta: o "usage" vem no último evento do
// streaming (ou no fim do JSON). Ler o fim depois de repassar não atrasa a
// pessoa, e nada do texto sai daqui além dos dois números.
type tailBuffer struct{ b []byte }

const tailSize = 16 << 10

func (t *tailBuffer) Write(p []byte) {
	t.b = append(t.b, p...)
	if len(t.b) > 2*tailSize {
		t.b = append(t.b[:0], t.b[len(t.b)-tailSize:]...)
	}
}

// Dentro do texto da resposta as aspas vêm escapadas (\"), então só o campo
// de verdade casa. O Groq antigo manda em x_groq.usage, que também casa.
var (
	promptRe     = regexp.MustCompile(`"prompt_tokens"\s*:\s*(\d+)`)
	completionRe = regexp.MustCompile(`"completion_tokens"\s*:\s*(\d+)`)
)

func (t *tailBuffer) usage() (in, out int) { return lastInt(promptRe, t.b), lastInt(completionRe, t.b) }

func lastInt(re *regexp.Regexp, b []byte) int {
	all := re.FindAllSubmatch(b, -1)
	if len(all) == 0 {
		return -1
	}
	n, err := strconv.Atoi(string(all[len(all)-1][1]))
	if err != nil {
		return -1
	}
	return n
}
