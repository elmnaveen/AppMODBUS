import { createRoot } from 'react-dom/client'
import App from './App'
import './styles.css'

// StrictMode is intentionally off: PollWindow tracks previous values in a
// ref during render to flash changed cells, which double rendering defeats.
createRoot(document.getElementById('root')!).render(<App />)
