package com.nocturneosint.app.data.remote

import com.nocturneosint.app.core.network.nocturneHttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.MockRequestHandleScope
import io.ktor.client.engine.mock.respond
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.client.request.HttpRequestData
import io.ktor.client.request.HttpResponseData
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.IOException

/** A NocturneApi whose requests are answered by [handler] instead of the network. */
internal fun apiAnswering(handler: suspend MockRequestHandleScope.(HttpRequestData) -> HttpResponseData) =
    NocturneApi(nocturneHttpClient("http://backend.test/", MockEngine(handler)))

internal fun MockRequestHandleScope.json(body: String, status: HttpStatusCode = HttpStatusCode.OK) =
    respond(body, status, headersOf(HttpHeaders.ContentType, "application/json"))

class NocturneApiTest {
    @Test
    fun `parses the status body and requests paths under the base URL`() = runTest {
        var requested = ""
        val api = apiAnswering { request ->
            requested = request.url.toString()
            json("""{"status":"ok","ignored":true}""")
        }
        assertEquals(ApiResult.Success(StatusResponse("ok")), api.health())
        assertEquals("http://backend.test/health", requested)
    }

    @Test
    fun `non-2xx responses become Http errors with their code`() = runTest {
        for (code in listOf(404, 500, 503)) {
            val api = apiAnswering { json("""{"status":"unavailable"}""", HttpStatusCode.fromValue(code)) }
            assertEquals(ApiResult.Failure(ApiError.Http(code)), api.ready())
        }
    }

    @Test
    fun `malformed or unexpected bodies are UnexpectedResponse`() = runTest {
        for (body in listOf("{", """{"state":"ok"}""")) { // truncated JSON; missing field
            assertEquals(ApiResult.Failure(ApiError.UnexpectedResponse), apiAnswering { json(body) }.health())
        }
        val html = apiAnswering { respond("<html></html>", HttpStatusCode.OK, headersOf(HttpHeaders.ContentType, "text/html")) }
        assertEquals(ApiResult.Failure(ApiError.UnexpectedResponse), html.health())
    }

    @Test
    fun `transport failures map to NoConnection and Timeout`() = runTest {
        assertEquals(ApiResult.Failure(ApiError.NoConnection), apiAnswering { throw IOException("connection refused") }.health())
        assertEquals(
            ApiResult.Failure(ApiError.Timeout),
            apiAnswering { throw HttpRequestTimeoutException("http://backend.test/health", 10_000) }.health(),
        )
    }
}
