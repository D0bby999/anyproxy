// SPDX-License-Identifier: MIT
import React from 'react'
import ReactDOM from 'react-dom/client'
import '@fontsource-variable/inter'
import './styles/tokens.css'
import './styles/base.css'
import './styles/shell.css'
import { applyTheme, readTheme } from './lib/theme'
import App from './App'

applyTheme(readTheme())

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
