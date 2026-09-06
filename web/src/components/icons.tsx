
const base = { fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }

export const IcFile = ({ stroke, size = 14 }: { stroke?: string; size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" {...base} stroke={stroke || 'currentColor'}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></svg>
)
export const IcChevron = ({ size = 14 }: { size?: number }) => <svg width={size} height={size} viewBox="0 0 24 24" {...base}><polyline points="6 9 12 15 18 9" /></svg>
export const IcBranch = ({ size = 12 }: { size?: number }) => <svg width={size} height={size} viewBox="0 0 24 24" {...base}><line x1="6" y1="3" x2="6" y2="15" /><circle cx="18" cy="6" r="3" /><circle cx="6" cy="18" r="3" /><path d="M18 9a9 9 0 0 1-9 9" /></svg>
export const IcSearch = ({ size = 14 }: { size?: number }) => <svg width={size} height={size} viewBox="0 0 24 24" {...base}><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
export const IcCheck = ({ size = 12 }: { size?: number }) => <svg width={size} height={size} viewBox="0 0 24 24" {...base}><polyline points="20 6 9 17 4 12" /></svg>
export const IcSend = () => <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" /></svg>
export const IcPlus = ({ size = 12 }: { size?: number }) => <svg width={size} height={size} viewBox="0 0 24 24" {...base}><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
export const IcPanelLeft = ({ size = 14 }: { size?: number }) => <svg width={size} height={size} viewBox="0 0 24 24" {...base}><rect x="3" y="3" width="18" height="18" rx="2" /><line x1="9" y1="3" x2="9" y2="21" /></svg>
export const IcBrand = () => <svg width="12" height="12" viewBox="0 0 24 24" {...base}><path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" /></svg>
export const IcCircleCheck = () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--method-get)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><polyline points="22 4 12 14.01 9 11.01" /></svg>
export const IcCircleX = () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--method-del)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10" /><line x1="15" y1="9" x2="9" y2="15" /><line x1="9" y1="9" x2="15" y2="15" /></svg>
export const IcCircleDot = () => <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--text-muted)" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="10" /></svg>

export function AgentAvatar({ size = 20, radius = 4 }: { size?: number; radius?: number }) {
  const id = 'wv'
  return (
    <svg width={size} height={size} viewBox="0 0 100 100" fill="none" aria-hidden="true" style={{ borderRadius: radius, display: 'block' }}>
      <defs>
        <g id={`calm-${id}`}><path d="M0 16c28 0 52-6 84-5s72 10 116 5v114H0Z" fill="#fff" /></g>
        <g id={`surge-${id}`}><path d="M0 6c24 0 44 4 62 14 16 8 34 8 52 2 20-7 38-12 54-12 12 0 24 4 32 6v114H0Z" fill="#fff" /></g>
        <g id={`roll-${id}`}><path d="M0 8c12 0 23 16 35 16S58 8 70 8s23 16 35 16 23-16 35-16 23 16 35 16c8 0 17-8 25-10v116H0Z" fill="#fff" /></g>
        <g id={`body-${id}`}>
          <g opacity=".14"><use transform="translate(-50) translate(23.4058,-4.39192)" href={`#calm-${id}`} /></g>
          <g opacity=".24"><use transform="translate(-50 17) translate(-11.327,2.70725)" href={`#surge-${id}`} /></g>
          <g opacity=".34"><use transform="translate(-50 34) translate(-13.9132,3.19657)" href={`#surge-${id}`} /></g>
          <g opacity=".42"><use transform="translate(-50 51) translate(23.3774,-4.70873)" href={`#roll-${id}`} /></g>
          <g opacity=".48"><use transform="translate(-50 68) translate(-18.989,-2.52746)" href={`#calm-${id}`} /></g>
        </g>
        <g id={`rot-${id}`}><use transform="rotate(-139.879,50,50)" href={`#body-${id}`} /></g>
        <clipPath id={`clip-${id}`}><rect width="100" height="100" rx="0" ry="0" /></clipPath>
      </defs>
      <g clipPath={`url(#clip-${id})`}><rect width="100" height="100" fill="#1d4ed8" /><use href={`#rot-${id}`} /></g>
    </svg>
  )
}
