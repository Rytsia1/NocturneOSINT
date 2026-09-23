package com.nocturneosint.app.data.remote

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.network.sockets.ConnectTimeoutException
import io.ktor.client.network.sockets.SocketTimeoutException
import io.ktor.client.plugins.HttpRequestTimeoutException
import io.ktor.client.request.get
import io.ktor.http.isSuccess
import kotlinx.coroutines.CancellationException
import kotlinx.serialization.Serializable
import java.io.IOException

/** Body of GET /health ({"status":"ok"}) and GET /ready ({"status":"ready"|"unavailable"}). */
@Serializable
data class StatusResponse(val status: String)

sealed interface ApiResult<out T> {
    data class Success<T>(val value: T) : ApiResult<T>
    data class Failure(val error: ApiError) : ApiResult<Nothing>
}

/** Transport-level failures; never raw exceptions or stack traces. */
sealed interface ApiError {
    data object NoConnection : ApiError
    data object Timeout : ApiError
    data class Http(val code: Int) : ApiError
    data object UnexpectedResponse : ApiError
}

/** Calls to the Nocturne REST API. Only the connectivity endpoints exist so far. */
class NocturneApi(private val client: HttpClient) {
    suspend fun health(): ApiResult<StatusResponse> = getStatus("health")

    suspend fun ready(): ApiResult<StatusResponse> = getStatus("ready")

    private suspend fun getStatus(path: String): ApiResult<StatusResponse> = try {
        val response = client.get(path)
        if (response.status.isSuccess()) {
            ApiResult.Success(response.body<StatusResponse>())
        } else {
            ApiResult.Failure(ApiError.Http(response.status.value))
        }
    } catch (e: HttpRequestTimeoutException) {
        ApiResult.Failure(ApiError.Timeout)
    } catch (e: ConnectTimeoutException) {
        ApiResult.Failure(ApiError.Timeout)
    } catch (e: SocketTimeoutException) {
        ApiResult.Failure(ApiError.Timeout)
    } catch (e: CancellationException) {
        throw e
    } catch (e: IOException) {
        ApiResult.Failure(ApiError.NoConnection)
    } catch (e: Exception) {
        // Wrong content type, malformed JSON, missing fields.
        ApiResult.Failure(ApiError.UnexpectedResponse)
    }
}
