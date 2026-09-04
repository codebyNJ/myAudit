import { useEffect, useState } from 'react'
import './splash.css'

// Cinematic sea-wave loading screen (from the provided design), shown on boot,
// then fades to reveal the app.
export function Splash({ onDone }: { onDone: () => void }) {
  const [exiting, setExiting] = useState(false)
  useEffect(() => {
    const t1 = setTimeout(() => setExiting(true), 2600)
    const t2 = setTimeout(onDone, 3400)
    return () => { clearTimeout(t1); clearTimeout(t2) }
  }, [onDone])

  return (
    <div id="splash" className={exiting ? 'exiting' : ''}>
      <div className="noise-overlay" />
      <div className="splash-glow" />
      <div className="sea-wave wave-back" />
      <div className="sea-wave wave-mid" />
      <div className="splash-brand">
        <div className="splash-logo-container">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100" fill="none" aria-hidden="true">
            <defs>
              <g id="s-calm"><path d="M0 16c28 0 52-6 84-5s72 10 116 5v114H0Z" fill="#ffffff" /></g>
              <g id="s-surge"><path d="M0 6c24 0 44 4 62 14 16 8 34 8 52 2 20-7 38-12 54-12 12 0 24 4 32 6v114H0Z" fill="#ffffff" /></g>
              <g id="s-roll"><path d="M0 8c12 0 23 16 35 16S58 8 70 8s23 16 35 16 23-16 35-16 23 16 35 16c8 0 17-8 25-10v116H0Z" fill="#ffffff" /></g>
              <g id="s-body">
                <g opacity=".14"><use transform="translate(-50) translate(23.4,-4.39)" href="#s-calm" /></g>
                <g opacity=".24"><use transform="translate(-50 17) translate(-11.3,2.7)" href="#s-surge" /></g>
                <g opacity=".34"><use transform="translate(-50 34) translate(-13.9,3.19)" href="#s-surge" /></g>
                <g opacity=".42"><use transform="translate(-50 51) translate(23.3,-4.7)" href="#s-roll" /></g>
                <g opacity=".48"><use transform="translate(-50 68) translate(-18.9,-2.52)" href="#s-calm" /></g>
              </g>
              <g id="s-rot"><use transform="rotate(-139.879,50,50)" href="#s-body" /></g>
              <clipPath id="s-clip"><rect width="100" height="100" /></clipPath>
            </defs>
            <g clipPath="url(#s-clip)"><use href="#s-rot" /></g>
          </svg>
        </div>
        <div className="splash-text">myIntern</div>
        <div className="splash-loading-bar"><div className="splash-progress" /></div>
      </div>
      <div className="sea-wave wave-front" />
    </div>
  )
}
