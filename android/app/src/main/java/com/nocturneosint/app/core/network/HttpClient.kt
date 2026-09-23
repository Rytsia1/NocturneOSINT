package com.nocturneosint.app.core.network

import io.ktor.client.HttpClient
import io.ktor.client.engine.HttpClientEngine
import io.ktor.client.engine.android.Android
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.plugins.defaultRequest
import io.ktor.client.plugins.logging.ANDROID
import io.ktor.client.plugins.logging.LogLevel
import io.ktor.client.plugins.logging.Logger
import io.ktor.client.plugins.logging.Logging
import io.ktor.serialization.kotlinx.json.json
import kotlinx.serialization.json.Json

/**
 * The single HTTP client for the Nocturne REST API. Relative request paths
 * resolve against [baseUrl]; non-2xx responses are returned, not thrown, so
 * callers map them to results. [logRequests] is meant for debug builds only.
 */
fun nocturneHttpClient(
    baseUrl: String,
    engine: HttpClientEngine = Android.create(),
    logRequests: Boolean = false,
): HttpClient = HttpClient(engine) {
    expectSuccess = false
    install(ContentNegotiation) { json(Json { ignoreUnknownKeys = true }) }
    install(HttpTimeout) {
        connectTimeoutMillis = 5_000
        requestTimeoutMillis = 10_000
    }
    if (logRequests) {
        install(Logging) {
            logger = Logger.ANDROID
            level = LogLevel.INFO
        }
    }
    defaultRequest { url(baseUrl) }
}
