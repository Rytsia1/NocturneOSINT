package com.nocturneosint.app.data.repository

import com.nocturneosint.app.data.remote.ApiError
import com.nocturneosint.app.data.remote.ApiResult
import com.nocturneosint.app.data.remote.NocturneApi
import com.nocturneosint.app.domain.model.BackendStatus
import com.nocturneosint.app.domain.model.ConnectionProblem
import com.nocturneosint.app.domain.repository.BackendRepository

/**
 * GET /health says whether the API process answers; GET /ready whether its
 * database is reachable (503 when not). The backend's semantics are used as-is.
 */
class RemoteBackendRepository(private val api: NocturneApi) : BackendRepository {
    override suspend fun status(): BackendStatus {
        when (val health = api.health()) {
            is ApiResult.Failure -> return BackendStatus.Unreachable(health.error.toProblem())
            is ApiResult.Success -> if (health.value.status != "ok") {
                return BackendStatus.Unreachable(ConnectionProblem.UnexpectedResponse)
            }
        }
        return when (val ready = api.ready()) {
            is ApiResult.Success -> if (ready.value.status == "ready") {
                BackendStatus.Ready
            } else {
                BackendStatus.Unreachable(ConnectionProblem.UnexpectedResponse)
            }
            is ApiResult.Failure -> if (ready.error == ApiError.Http(503)) {
                BackendStatus.DatabaseUnavailable
            } else {
                BackendStatus.Unreachable(ready.error.toProblem())
            }
        }
    }
}

private fun ApiError.toProblem(): ConnectionProblem = when (this) {
    ApiError.NoConnection -> ConnectionProblem.NoConnection
    ApiError.Timeout -> ConnectionProblem.Timeout
    ApiError.UnexpectedResponse -> ConnectionProblem.UnexpectedResponse
    is ApiError.Http -> if (code >= 500) ConnectionProblem.ServerError else ConnectionProblem.ClientError
}
