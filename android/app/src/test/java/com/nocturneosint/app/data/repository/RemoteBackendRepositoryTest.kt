package com.nocturneosint.app.data.repository

import com.nocturneosint.app.data.remote.apiAnswering
import com.nocturneosint.app.data.remote.json
import com.nocturneosint.app.domain.model.BackendStatus
import com.nocturneosint.app.domain.model.ConnectionProblem
import io.ktor.http.HttpStatusCode
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.IOException

class RemoteBackendRepositoryTest {
    /** Answers /health and /ready as the Go backend does; null = connection refused. */
    private fun repository(health: Pair<Int, String>?, ready: Pair<Int, String>?) = RemoteBackendRepository(
        apiAnswering { request ->
            val (code, body) = (if (request.url.encodedPath == "/health") health else ready)
                ?: throw IOException("connection refused")
            json(body, HttpStatusCode.fromValue(code))
        },
    )

    private val ok = 200 to """{"status":"ok"}"""
    private val ready = 200 to """{"status":"ready"}"""

    @Test
    fun `health and ready answering means Ready`() = runTest {
        assertEquals(BackendStatus.Ready, repository(ok, ready).status())
    }

    @Test
    fun `ready 503 means the API is up but its database is not`() = runTest {
        assertEquals(BackendStatus.DatabaseUnavailable, repository(ok, 503 to """{"status":"unavailable"}""").status())
    }

    @Test
    fun `failures map to connection problems`() = runTest {
        val cases = listOf(
            repository(null, null) to ConnectionProblem.NoConnection,
            repository(404 to "{}", ready) to ConnectionProblem.ClientError,
            repository(500 to "{}", ready) to ConnectionProblem.ServerError,
            repository(ok, 500 to "{}") to ConnectionProblem.ServerError,
            repository(200 to """{"status":"degraded"}""", ready) to ConnectionProblem.UnexpectedResponse,
            repository(ok, 200 to """{"status":"maybe"}""") to ConnectionProblem.UnexpectedResponse,
        )
        for ((repo, problem) in cases) {
            assertEquals(BackendStatus.Unreachable(problem), repo.status())
        }
    }
}
