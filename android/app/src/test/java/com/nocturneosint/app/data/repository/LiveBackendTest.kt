package com.nocturneosint.app.data.repository

import com.nocturneosint.app.core.network.nocturneHttpClient
import com.nocturneosint.app.data.remote.NocturneApi
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assume.assumeTrue
import org.junit.Test

/**
 * Talks to a real Nocturne backend through the app's own client, repository
 * and Ktor Android engine. Skipped unless NOCTURNE_BACKEND_URL is set, e.g.
 *   NOCTURNE_BACKEND_URL=http://localhost:8080/ NOCTURNE_BACKEND_EXPECT=Ready ./gradlew test
 */
class LiveBackendTest {
    @Test
    fun `reports the live backend status`() {
        val url = System.getenv("NOCTURNE_BACKEND_URL")
        assumeTrue("NOCTURNE_BACKEND_URL not set; skipping live backend test", !url.isNullOrBlank())

        val status = runBlocking { RemoteBackendRepository(NocturneApi(nocturneHttpClient(url!!))).status() }
        println("Live backend $url: $status")
        assertEquals(System.getenv("NOCTURNE_BACKEND_EXPECT") ?: "Ready", status.toString())
    }
}
