package llm

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// readLines chama fn para cada linha não vazia de r. Serve para NDJSON (Ollama)
// e Server-Sent Events (OpenAI). Linhas de até 1 MB são aceitas.
func readLines(r io.Reader, fn func(line []byte) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			return err
		}
	}
	return sc.Err()
}

// thinkFilter remove trechos <think>...</think> que alguns modelos abertos
// (Qwen3, DeepSeek-R1) escrevem antes da resposta, mesmo em streaming, onde a
// tag pode chegar partida entre pedaços.
type thinkFilter struct {
	inside  bool
	pending string // fim do texto que pode ser o começo de uma tag
	out     strings.Builder
}

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// Write recebe um pedaço e devolve a parte segura para mostrar ao usuário.
func (f *thinkFilter) Write(chunk string) string {
	s := f.pending + chunk
	f.pending = ""
	var visible strings.Builder
	for s != "" {
		tag := thinkOpen
		if f.inside {
			tag = thinkClose
		}
		if i := strings.Index(s, tag); i >= 0 {
			if !f.inside {
				visible.WriteString(s[:i])
			}
			s = s[i+len(tag):]
			f.inside = !f.inside
			continue
		}
		// Guarda um possível começo de tag para o próximo pedaço.
		keep := partialSuffix(s, tag)
		if !f.inside {
			visible.WriteString(s[:len(s)-keep])
		}
		f.pending = s[len(s)-keep:]
		break
	}
	out := visible.String()
	if f.out.Len() == 0 {
		out = strings.TrimLeft(out, "\n ") // o modelo costuma pular linhas após </think>
	}
	f.out.WriteString(out)
	return out
}

// Flush devolve o que sobrou no buffer ao fim do stream.
func (f *thinkFilter) Flush() string {
	rest := ""
	if !f.inside {
		rest = f.pending
	}
	f.pending = ""
	f.out.WriteString(rest)
	return rest
}

// Text é tudo o que foi liberado para o usuário.
func (f *thinkFilter) Text() string { return strings.TrimSpace(f.out.String()) }

// partialSuffix devolve o tamanho do maior sufixo de s que é prefixo de tag.
func partialSuffix(s, tag string) int {
	for n := min(len(tag)-1, len(s)); n > 0; n-- {
		if strings.HasSuffix(s, tag[:n]) {
			return n
		}
	}
	return 0
}

// streamText liga um thinkFilter ao callback do usuário.
func streamText(onText func(string)) (emit func(string), finish func() string) {
	f := &thinkFilter{}
	emit = func(chunk string) {
		if out := f.Write(chunk); out != "" {
			onText(out)
		}
	}
	finish = func() string {
		if out := f.Flush(); out != "" {
			onText(out)
		}
		return f.Text()
	}
	return emit, finish
}
