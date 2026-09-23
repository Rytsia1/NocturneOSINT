package com.nocturneosint.app.presentation.home

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import com.nocturneosint.app.domain.model.BackendStatus
import com.nocturneosint.app.domain.model.ConnectionProblem
import com.nocturneosint.app.presentation.theme.NocturneColors
import com.nocturneosint.app.presentation.theme.Spacing

/** Placeholder home: shows that the client runs and whether the backend is reachable. */
@Composable
fun HomeScreen(state: HomeUiState, onRetry: () -> Unit) {
    Surface(color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxSize()) {
        Column(
            modifier = Modifier.safeDrawingPadding().padding(Spacing.m),
            verticalArrangement = Arrangement.spacedBy(Spacing.m),
        ) {
            Text("Nocturne", style = MaterialTheme.typography.headlineMedium)
            Text(
                "Client foundation initialized.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            val (label, color) = describe(state)
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Spacing.s)) {
                // Colour is never the only signal: the label always states the status.
                Surface(color = color, shape = CircleShape, modifier = Modifier.size(Spacing.s)) {}
                Text(label, style = MaterialTheme.typography.bodyLarge)
            }
            if (state is HomeUiState.Checked) {
                OutlinedButton(onClick = onRetry) { Text("Check again") }
            }
        }
    }
}

private fun describe(state: HomeUiState): Pair<String, Color> = when (state) {
    HomeUiState.Checking -> "Checking backend…" to NocturneColors.TextTertiary
    is HomeUiState.Checked -> when (val status = state.status) {
        BackendStatus.Ready -> "Backend reachable · database ready" to NocturneColors.Success
        BackendStatus.DatabaseUnavailable -> "Backend reachable · database unavailable" to NocturneColors.Warning
        is BackendStatus.Unreachable -> "Backend unreachable: " + when (status.problem) {
            ConnectionProblem.NoConnection -> "no connection to the server"
            ConnectionProblem.Timeout -> "the server did not respond in time"
            ConnectionProblem.ClientError -> "the server rejected the request"
            ConnectionProblem.ServerError -> "the server reported an error"
            ConnectionProblem.UnexpectedResponse -> "unexpected response from the server"
        } to NocturneColors.Error
    }
}
