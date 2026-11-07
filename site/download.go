package site

import (
	"net/http"

	"rsc.io/qr"
)

// apkBase é o endereço público do bucket do Cloudflare R2 onde a Release publica o app
// Android (android/finan_rich.apk e android/latest.json; veja docs/DISTRIBUICAO.md). Não
// é segredo. Vazio: o download ainda não está no ar.
var apkBase = ""

// DownloadPaths são os caminhos de Download. O cmd/api registra um a um: com a pasta
// "/baixar/" inteira, o Go redirecionaria a página /baixar para /baixar/.
var DownloadPaths = []string{"/baixar/android", "/baixar/versao", "/baixar/qr.png"}

// Download serve as rotas de /baixar/ (a página /baixar é o baixar.html):
//   - /baixar/android: o APK mais recente (redireciona para o R2);
//   - /baixar/versao: a versão mais recente, que o app confere para avisar de atualização;
//   - /baixar/qr.png: o QR code que abre /baixar no celular.
func Download() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /baixar/android", toBucket("/android/finan_rich.apk"))
	mux.HandleFunc("GET /baixar/versao", toBucket("/android/latest.json"))
	mux.HandleFunc("GET /baixar/qr.png", func(w http.ResponseWriter, r *http.Request) {
		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http" // só no teste local; a Vercel é sempre https
		}
		code, err := qr.Encode(scheme+"://"+r.Host+"/baixar", qr.M)
		if err != nil {
			http.Error(w, "QR code indisponível", http.StatusInternalServerError)
			return
		}
		code.Scale = 8
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(code.PNG())
	})
	return mux
}

func toBucket(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apkBase == "" {
			http.Error(w, "O download do app ainda não está no ar.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache") // a versão muda a cada publicação
		http.Redirect(w, r, apkBase+file, http.StatusFound)
	}
}
