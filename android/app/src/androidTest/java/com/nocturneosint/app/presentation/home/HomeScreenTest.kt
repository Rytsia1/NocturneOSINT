package com.nocturneosint.app.presentation.home

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.nocturneosint.app.domain.model.BackendStatus
import com.nocturneosint.app.domain.model.ConnectionProblem
import com.nocturneosint.app.presentation.theme.NocturneTheme
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test

class HomeScreenTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun rendersEachConnectionState() {
        var state: HomeUiState by mutableStateOf(HomeUiState.Checking)
        var retries = 0
        compose.setContent { NocturneTheme { HomeScreen(state, onRetry = { retries++ }) } }

        compose.onNodeWithText("Client foundation initialized.").assertIsDisplayed()
        compose.onNodeWithText("Checking backend…").assertIsDisplayed()

        state = HomeUiState.Checked(BackendStatus.Ready)
        compose.onNodeWithText("Backend reachable · database ready").assertIsDisplayed()

        state = HomeUiState.Checked(BackendStatus.DatabaseUnavailable)
        compose.onNodeWithText("Backend reachable · database unavailable").assertIsDisplayed()

        state = HomeUiState.Checked(BackendStatus.Unreachable(ConnectionProblem.Timeout))
        compose.onNodeWithText("Backend unreachable: the server did not respond in time").assertIsDisplayed()
        compose.onNodeWithText("Check again").performClick()
        assertEquals(1, retries)
    }
}
