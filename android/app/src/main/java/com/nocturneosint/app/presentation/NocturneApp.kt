package com.nocturneosint.app.presentation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.nocturneosint.app.presentation.home.HomeScreen
import com.nocturneosint.app.presentation.home.HomeViewModel
import kotlinx.serialization.Serializable

/** Type-safe navigation routes; later destinations are added here. */
@Serializable
object HomeRoute

@Composable
fun NocturneApp(navController: NavHostController = rememberNavController()) {
    NavHost(navController, startDestination = HomeRoute) {
        composable<HomeRoute> {
            val viewModel: HomeViewModel = viewModel(factory = HomeViewModel.Factory)
            val state by viewModel.state.collectAsStateWithLifecycle()
            HomeScreen(state, onRetry = viewModel::check)
        }
    }
}
