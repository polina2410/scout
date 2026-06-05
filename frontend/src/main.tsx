import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Provider } from 'react-redux'
import { MotionConfig } from 'motion/react'
import { store } from './store'
import './index.css'
import { App } from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Provider store={store}>
      {/* reducedMotion="user" makes every motion component honour
          prefers-reduced-motion (transforms disabled, opacity kept). */}
      <MotionConfig reducedMotion="user">
        <App />
      </MotionConfig>
    </Provider>
  </StrictMode>,
)
