package com.nocturneosint.app.domain.model

/** Whether the Nocturne backend can currently serve the app. */
sealed interface BackendStatus {
    /** The API answers and its database is reachable. */
    data object Ready : BackendStatus

    /** The API answers, but reports its database as unavailable. */
    data object DatabaseUnavailable : BackendStatus

    data class Unreachable(val problem: ConnectionProblem) : BackendStatus
}

enum class ConnectionProblem { NoConnection, Timeout, ClientError, ServerError, UnexpectedResponse }
