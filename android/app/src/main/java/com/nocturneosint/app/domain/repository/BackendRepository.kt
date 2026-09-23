package com.nocturneosint.app.domain.repository

import com.nocturneosint.app.domain.model.BackendStatus

interface BackendRepository {
    /** Never throws: every failure is a [BackendStatus.Unreachable]. */
    suspend fun status(): BackendStatus
}
