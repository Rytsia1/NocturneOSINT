package com.nocturneosint.app.presentation.home

import com.nocturneosint.app.domain.model.BackendStatus
import com.nocturneosint.app.domain.model.ConnectionProblem
import com.nocturneosint.app.domain.repository.BackendRepository
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

/** Suspends each status() call until the test completes it. */
private class FakeBackend : BackendRepository {
    val pending = ArrayDeque<CompletableDeferred<BackendStatus>>()
    override suspend fun status(): BackendStatus = CompletableDeferred<BackendStatus>().also { pending.addLast(it) }.await()
}

@OptIn(ExperimentalCoroutinesApi::class)
class HomeViewModelTest {
    private val dispatcher = StandardTestDispatcher()

    @Before
    fun setUp() = Dispatchers.setMain(dispatcher)

    @After
    fun tearDown() = Dispatchers.resetMain()

    @Test
    fun `checks on start, then shows the result`() = runTest(dispatcher) {
        val backend = FakeBackend()
        val viewModel = HomeViewModel(backend)
        assertEquals(HomeUiState.Checking, viewModel.state.value)

        advanceUntilIdle()
        backend.pending.removeFirst().complete(BackendStatus.Ready)
        advanceUntilIdle()
        assertEquals(HomeUiState.Checked(BackendStatus.Ready), viewModel.state.value)
    }

    @Test
    fun `failure is state, and check again starts over`() = runTest(dispatcher) {
        val backend = FakeBackend()
        val viewModel = HomeViewModel(backend)
        advanceUntilIdle()
        val unreachable = BackendStatus.Unreachable(ConnectionProblem.NoConnection)
        backend.pending.removeFirst().complete(unreachable)
        advanceUntilIdle()
        assertEquals(HomeUiState.Checked(unreachable), viewModel.state.value)

        viewModel.check()
        assertEquals(HomeUiState.Checking, viewModel.state.value)
        advanceUntilIdle()
        backend.pending.removeFirst().complete(BackendStatus.DatabaseUnavailable)
        advanceUntilIdle()
        assertEquals(HomeUiState.Checked(BackendStatus.DatabaseUnavailable), viewModel.state.value)
    }
}
