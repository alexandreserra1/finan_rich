import XCTest

final class AppUITests: XCTestCase {
    @MainActor
    func testAbreConfiguracoesSemConectarConta() {
        continueAfterFailure = false
        let app = XCUIApplication()
        app.launch()
        defer { app.terminate() }

        let webView = app.webViews.firstMatch
        XCTAssertTrue(webView.waitForExistence(timeout: 30), "O app deve abrir a tela web local.")

        let aviso = webView.staticTexts.matching(
            NSPredicate(format: "label CONTAINS %@", "ainda não está conectado")
        ).firstMatch
        XCTAssertTrue(aviso.waitForExistence(timeout: 30), "O primeiro uso deve informar que o celular ainda não está conectado.")

        let inicio = XCTAttachment(screenshot: app.screenshot())
        inicio.name = "Primeiro uso sem conexão"
        inicio.lifetime = .keepAlways
        add(inicio)

        let conectar = webView.links.matching(identifier: "Conectar agora").firstMatch
        XCTAssertTrue(conectar.waitForExistence(timeout: 15), "O aviso deve oferecer o link Conectar agora.")
        conectar.tap()

        let usarSozinho = webView.buttons.matching(identifier: "Usar só neste celular").firstMatch
        // No simulador do CI, o toque às vezes não vira clique e a tela fica parada (ENG-008):
        // sem os Ajustes em 15 s, e com o link ainda na tela, toca de novo antes de falhar.
        if !usarSozinho.waitForExistence(timeout: 15), conectar.exists {
            conectar.tap()
        }
        XCTAssertTrue(usarSozinho.waitForExistence(timeout: 30), "Os ajustes devem oferecer o uso somente neste celular.")

        let ajustes = XCTAttachment(screenshot: app.screenshot())
        ajustes.name = "Ajustes antes de conectar ou criar dados"
        ajustes.lifetime = .keepAlways
        add(ajustes)
    }
}
