package com.nocturneosint.app.presentation.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider.AndroidViewModelFactory.Companion.APPLICATION_KEY
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import com.nocturneosint.app.NocturneApplication
import com.nocturneosint.app.domain.model.BackendStatus
import com.nocturneosint.app.domain.repository.BackendRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed interface HomeUiState {
    data object Checking : HomeUiState
    data class Checked(val status: BackendStatus) : HomeUiState
}

/** Checks backend connectivity once on start, and again on [check]. */
class HomeViewModel(private val backend: BackendRepository) : ViewModel() {
    private val _state = MutableStateFlow<HomeUiState>(HomeUiState.Checking)
    val state: StateFlow<HomeUiState> = _state.asStateFlow()
    private var job: Job? = null

    init {
        check()
    }

    fun check() {
        job?.cancel()
        _state.value = HomeUiState.Checking
        job = viewModelScope.launch { _state.value = HomeUiState.Checked(backend.status()) }
    }

    companion object {
        val Factory = viewModelFactory {
            initializer { HomeViewModel((this[APPLICATION_KEY] as NocturneApplication).backendRepository) }
        }
    }
}
