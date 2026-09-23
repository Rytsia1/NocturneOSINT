package com.nocturneosint.app

import android.app.Application
import com.nocturneosint.app.core.network.nocturneHttpClient
import com.nocturneosint.app.data.remote.NocturneApi
import com.nocturneosint.app.data.repository.RemoteBackendRepository
import com.nocturneosint.app.domain.repository.BackendRepository

class NocturneApplication : Application() {
    // ponytail: manual wiring; add a DI framework when the object graph outgrows this.
    val backendRepository: BackendRepository by lazy {
        val client = nocturneHttpClient(BuildConfig.API_BASE_URL, logRequests = BuildConfig.DEBUG)
        RemoteBackendRepository(NocturneApi(client))
    }
}
