package web

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Contraste mínimo das cores do app.css (WCAG 2.1 AA), nos temas claro e
// escuro: 4,5:1 para texto e 3:1 para bordas de campos, barras e ícones.
// Mudou uma cor e o teste falhou? Escureça (tema claro) ou clareie (escuro)
// mantendo o tom, até passar.
func TestColorContrast(t *testing.T) {
	css, err := staticFS.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	light, dark := cssThemes(t, string(css))

	text := [][2]string{
		{"accent", "surface"}, {"accent", "bg"}, {"pos", "surface"}, {"neg", "surface"}, {"neg", "neg-soft"},
		{"warn-text", "warn-soft"}, {"accent-text", "accent"}, {"accent-strong", "accent-soft"},
		{"hero-text", "hero"}, {"hero-muted", "hero"}, {"hero-pos", "hero"}, {"hero-neg", "hero"},
		{"neg-text", "neg"},
	}
	for _, fg := range []string{"text", "text-2", "muted"} {
		for _, bg := range []string{"bg", "surface", "surface-2"} {
			text = append(text, [2]string{fg, bg})
		}
	}
	graphics := [][2]string{{"border-input", "surface"}, {"border-input", "bg"}, {"border-input", "surface-2"}}
	for _, c := range []string{"necessidades", "desejos", "investimentos", "renda", "in", "out", "warn", "neg", "pos"} {
		graphics = append(graphics, [2]string{c, "surface"}, [2]string{c, "surface-2"})
	}
	for _, c := range []string{"necessidades", "desejos", "investimentos", "renda"} {
		graphics = append(graphics, [2]string{c, "track"}) // barra sobre o trilho
	}

	for _, theme := range []struct {
		name   string
		tokens map[string]string
	}{{"claro", light}, {"escuro", dark}} {
		check := func(pairs [][2]string, min float64, kind string) {
			for _, p := range pairs {
				fg, bg := theme.tokens[p[0]], theme.tokens[p[1]]
				if fg == "" || bg == "" {
					t.Errorf("tema %s: cor --%s ou --%s não encontrada", theme.name, p[0], p[1])
					continue
				}
				if r := contrastRatio(fg, bg); r < min {
					t.Errorf("tema %s, %s: --%s (%s) sobre --%s (%s) = %.2f:1, mínimo %.1f:1", theme.name, kind, p[0], fg, p[1], bg, r, min)
				}
			}
		}
		check(text, 4.5, "texto")
		check(graphics, 3, "borda/gráfico")
	}
}

var cssToken = regexp.MustCompile(`--([\w-]+):\s*(#[0-9a-fA-F]{6})\b`)

// cssThemes lê as cores do :root (claro) e do bloco :root[data-theme="dark"],
// que redefine parte delas (o js/theme.js põe o data-theme no <html>).
func cssThemes(t *testing.T, css string) (light, dark map[string]string) {
	t.Helper()
	i := strings.Index(css, ":root {")
	j := strings.Index(css, `:root[data-theme="dark"] {`)
	if i < 0 || j < i {
		t.Fatal("app.css sem :root ou sem o tema escuro")
	}
	parse := func(block string, into map[string]string) {
		for _, m := range cssToken.FindAllStringSubmatch(block, -1) {
			into[m[1]] = strings.ToLower(m[2])
		}
	}
	light, dark = map[string]string{}, map[string]string{}
	parse(css[i:j], light)
	for k, v := range light {
		dark[k] = v
	}
	end := strings.Index(css[j:], "\n}")
	if end < 0 {
		t.Fatal("bloco do tema escuro sem fim")
	}
	parse(css[j:j+end], dark)
	return light, dark
}

func contrastRatio(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance é a luminância relativa de uma cor #rrggbb (definição da WCAG).
func luminance(hex string) float64 {
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		s := float64(v) / 255
		if s <= 0.03928 {
			c[i] = s / 12.92
		} else {
			c[i] = math.Pow((s+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
}
