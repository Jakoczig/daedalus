import { useState, useEffect } from 'react'

const MIN_PERCENT = 10

const CONTENT_LEVEL_LABELS = {
  High: 'RICH',
  Medium: 'ORDINARY',
  Low: 'BARREN',
  Unknown: '?'
}

// 2-letter abbreviations for known mining commodities (key = normalized lowercase no-spaces)
const MINING_ABBREV = {
  platinum: 'Pt', gold: 'Au', silver: 'Ag', palladium: 'Pd',
  painite: 'Pn', tritium: 'Tr',
  lowtemperaturediamond: 'LT', voidopals: 'VO',
  alexandrite: 'Al', grandidierite: 'Gd', benitoite: 'Bn',
  musgravite: 'Mu', monazite: 'Mo', rhodplumsite: 'Rh',
  serendibite: 'Sd', bromellite: 'Br', osmium: 'Os',
  uranium: 'Ur', gallium: 'Ga', indium: 'In',
  praseodymium: 'Pr', samarium: 'Sm', lanthanum: 'La',
  cerium: 'Ce', neodymium: 'Nd', yttrium: 'Yt',
  zirconium: 'Zr', niobium: 'Nb', molybdenum: 'Mb',
  copper: 'Cu', lithium: 'Li', beryllium: 'Be',
  rutile: 'Ru', bauxite: 'Bx', bertrandite: 'Bt',
  cobalt: 'Cb', taaffeite: 'Ta', jadeite: 'Jd',
  goslarite: 'Gs', coltan: 'Cn', indite: 'Id',
}

// Maps normalized key → proper display name for compound/tricky material names
const MATERIAL_DISPLAY = {
  lowtemperaturediamond: 'Low Temp. Diamond',
  voidopals: 'Void Opals',
  musgravite: 'Musgravite',
  rhodplumsite: 'Rhodplumsite',
  serendibite: 'Serendibite',
  grandidierite: 'Grandidierite',
  benitoite: 'Benitoite',
  alexandrite: 'Alexandrite',
  bertrandite: 'Bertrandite',
  praseodymium: 'Praseodymium',
}

function formatMaterialName (name) {
  if (!name) return ''
  // Handle localized format: "$LowTemperatureDiamond_Name;" → key
  const loc = name.match(/^\$(.+?)_[Nn]ame;?$/)
  const key = (loc ? loc[1] : name).toLowerCase().replace(/\s+/g, '')
  return MATERIAL_DISPLAY[key] ?? (name.charAt(0).toUpperCase() + name.slice(1))
}

// Animates bar width from 0 on mount/value change
function AnimatedBar ({ pct, variant = 'info' }) {
  const [width, setWidth] = useState(0)
  useEffect(() => {
    const id = requestAnimationFrame(() => setWidth(pct))
    return () => cancelAnimationFrame(id)
  }, [pct])
  return (
    <div className='mining-panel__bar-track'>
      <div
        className={`mining-panel__bar-fill mining-panel__bar-fill--${variant}`}
        style={{ width: `${width}%`, transition: 'width .55s cubic-bezier(0.4, 0, 0.2, 1)' }}
      />
    </div>
  )
}

function MiningCargoBay ({ cargo }) {
  const { max = 0, items = [], limpetCount = 0 } = cargo ?? {}
  const effectiveMax = max > 0 ? max : 36
  const numSections = Math.max(4, Math.ceil(effectiveMax / 9))

  // Optimal layout: try every column count, pick the one that maximises square size.
  // All values in rem, matching the 110×56rem dashboard coordinate space.
  // Dashboard padding: 1rem V each side. Bay padding: .8rem V, 1rem H.
  // Title row (in-flow): 1.5rem height + .35rem margin = 1.85rem.
  const AVAIL_H = 56 - 2 * 1 - 2 * 0.8 - 1.85  // ≈ 50.5 rem
  const MAX_W   = 27                              // max width for the sections area (rem)
  const SGAP    = 0.35                            // gap between sections (rem)
  const SQGAP   = 0.1                             // gap between squares within a section (rem)
  const MAX_SQ  = 5                               // cap on individual square size (rem)

  let bestCols = 1
  let bestSq   = 0
  for (let cols = 1; cols <= numSections; cols++) {
    const rows   = Math.ceil(numSections / cols)
    const sectH  = (AVAIL_H - (rows - 1) * SGAP) / rows
    const sectW  = (MAX_W   - (cols - 1) * SGAP) / cols
    const sectSz = Math.min(sectH, sectW)
    if (sectSz <= 0) continue
    const sq = Math.min(MAX_SQ, (sectSz - 2 * SQGAP) / 3)
    if (sq > bestSq) { bestSq = sq; bestCols = cols }
  }

  const sqSz   = bestSq
  const sectSz = sqSz * 3 + 2 * SQGAP

  const tonsArray = []
  for (let i = 0; i < limpetCount; i++) tonsArray.push('limpet')
  for (const { key, count } of items) {
    const slotType = MINING_ABBREV[key] ? key : 'generic'
    for (let i = 0; i < count; i++) tonsArray.push(slotType)
  }

  const totalSlots = numSections * 9
  const slots    = [...tonsArray, ...Array(Math.max(0, totalSlots - tonsArray.length)).fill(null)]
  const sections = Array.from({ length: numSections }, (_, s) => slots.slice(s * 9, (s + 1) * 9))

  const titleLabel  = max > 0 ? `CARGO BAY · ${max}T` : 'CARGO BAY'
  const abbrevSize  = `${Math.max(0.55, sqSz * 0.22).toFixed(2)}rem`
  const squareStyle = { width: `${sqSz}rem`, height: `${sqSz}rem` }
  const sectStyle   = {
    gridTemplateColumns: `repeat(3, ${sqSz}rem)`,
    gridTemplateRows:    `repeat(3, ${sqSz}rem)`,
    gap: `${SQGAP}rem`
  }
  const gridStyle = {
    gridTemplateColumns: `repeat(${bestCols}, ${sectSz}rem)`,
    gap: `${SGAP}rem`
  }

  return (
    <div className='mining-panel__cargo-bay'>
      <span className='mining-panel__cargo-corner mining-panel__cargo-corner--tl' />
      <span className='mining-panel__cargo-corner mining-panel__cargo-corner--tr' />
      <span className='mining-panel__cargo-corner mining-panel__cargo-corner--bl' />
      <span className='mining-panel__cargo-corner mining-panel__cargo-corner--br' />
      <div className='mining-panel__cargo-title'>{titleLabel}</div>
      <div className='mining-panel__cargo-sections-wrap'>
        <div className='mining-panel__cargo-sections' style={gridStyle}>
          {sections.map((section, s) => (
            <div key={s} className='mining-panel__cargo-section' style={sectStyle}>
              {section.map((content, i) => (
                <div
                  key={i}
                  className={`mining-panel__cargo-sq ${
                    !content ? 'mining-panel__cargo-sq--empty'
                    : content === 'limpet' ? 'mining-panel__cargo-sq--limpet'
                    : content === 'generic' ? 'mining-panel__cargo-sq--generic'
                    : 'mining-panel__cargo-sq--material'
                  }`}
                  style={squareStyle}
                >
                  {content && content !== 'limpet' && content !== 'generic' && (
                    <span className='mining-panel__cargo-abbrev' style={{ fontSize: abbrevSize }}>{MINING_ABBREV[content]}</span>
                  )}
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function MiningLaserPower ({ weaponPips }) {
  const pipsOk = !weaponPips || weaponPips.isFull
  const halfPips = weaponPips?.halfPips ?? 8
  return (
    <div className={`mining-panel__laser-power ${pipsOk ? 'mining-panel__laser-power--ok' : 'mining-panel__laser-power--warn'}`}>
      <i className={`icon mining-panel__laser-icon ${pipsOk ? 'daedalus-terminal-barrel' : 'daedalus-terminal-warning'}`} />
      <span className='mining-panel__laser-text'>LASER POWER</span>
      <span className='mining-panel__laser-value'>{halfPips % 2 === 0 ? halfPips / 2 : (halfPips / 2).toFixed(1)} / 4</span>
    </div>
  )
}

function MiningLimpetColumn ({ limpets, cargo }) {
  const { remaining = 0, warnNoLimpets = false } = limpets ?? {}
  const { max: cargoMax = 0, current: cargoCurrent = 0, limpetCount = 0 } = cargo ?? {}

  const otherCargo = Math.max(0, cargoCurrent - limpetCount)
  const otherPct = cargoMax > 0 ? (otherCargo / cargoMax) * 100 : 0
  const limpetPct = cargoMax > 0 ? (remaining / cargoMax) * 100 : 0
  const warn = warnNoLimpets || remaining < 10

  return (
    <div className='mining-panel__limpet-column'>
      <div className={`mining-panel__limpet-bar-wrap ${warn ? 'mining-panel__limpet-bar-wrap--warn' : ''}`}>
        {otherPct > 0 && (
          <div className='mining-panel__other-cargo-fill' style={{ height: `${otherPct}%` }} />
        )}
        {limpetPct > 0 && (
          <div
            className={`mining-panel__limpet-fill ${warn ? 'mining-panel__limpet-fill--warn' : ''}`}
            style={{ height: `${limpetPct}%` }}
          />
        )}
      </div>
      <div className={`mining-panel__limpet-count ${warn ? 'text-danger' : 'text-info'}`}>
        {remaining}
      </div>
      <div className='mining-panel__limpet-label'>LIMPETS</div>
    </div>
  )
}

function formatTimeAgo (timestamp) {
  if (!timestamp) return '—'
  const diff = (Date.now() - new Date(timestamp).getTime()) / 1000
  if (diff < 60) return `${Math.round(diff)}s`
  if (diff < 3600) return `${Math.round(diff / 60)}m`
  return `${Math.floor(diff / 3600)}h`
}

function PieDonut ({ materials }) {
  const R = 40
  const CIRC = 2 * Math.PI * R
  const total = materials.reduce((s, m) => s + m.pct, 0)
  const rock = Math.max(0, 100 - total)
  const segs = [
    ...materials.map(m => ({
      pct: m.pct,
      color: m.isTarget ? 'var(--color-secondary)' : 'rgba(250,150,0,0.5)'
    })),
    ...(rock > 1 ? [{ pct: rock, color: 'rgba(120,110,110,0.55)' }] : [])
  ]
  let acc = 0
  return (
    <svg viewBox='0 0 120 120' className='mining-panel__pie-chart'>
      {segs.map((seg, i) => {
        const len = (seg.pct / 100) * CIRC * 0.96
        const offset = CIRC / 4 - acc
        acc += (seg.pct / 100) * CIRC
        return (
          <circle key={i} cx='60' cy='60' r={R}
            fill='none'
            style={{ stroke: seg.color, strokeWidth: 22, strokeDasharray: `${len} ${CIRC}`, strokeDashoffset: offset }}
          />
        )
      })}
    </svg>
  )
}

function MiningRecentProspect ({ result, isOffline, notchCorner = null }) {
  const notchClass = notchCorner === 'bottom-right' ? 'mining-panel__prospector-card--notch-br'
    : notchCorner === 'top-right' ? 'mining-panel__prospector-card--notch-tr'
    : ''

  if (!result) {
    return (
      <div className={`mining-panel__prospector-card ${notchClass}`}>
        <div className='mining-panel__prospector-title'>RECENT PROSPECT</div>
        <div className={`mining-panel__prospector-empty ${isOffline ? 'mining-panel__prospector-empty--offline' : 'mining-panel__prospector-empty--ready'}`}>
          {isOffline ? 'OFFLINE' : 'READY'}
        </div>
      </div>
    )
  }

  const { materials = [], contentLevel, remaining, isGood, motherlode } = result
  const contentLabel = CONTENT_LEVEL_LABELS[contentLevel] ?? contentLevel
  const total = materials.reduce((s, m) => s + m.pct, 0)
  const rock = Math.max(0, 100 - total)

  return (
    <div className={`mining-panel__prospector-card ${isGood ? 'mining-panel__prospector-card--good' : ''} ${notchClass}`}>
      <div className='mining-panel__prospector-title'>
        <span>RECENT PROSPECT</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: '.4rem' }}>
          <span className={`mining-panel__content-badge mining-panel__content-badge--${contentLevel}`}>{contentLabel}</span>
          {remaining < 100 && <span style={{ opacity: 0.5, fontSize: '.9rem' }}>{Math.round(remaining)}%</span>}
        </div>
      </div>
      {motherlode && (
        <div style={{ fontSize: '.95rem', color: 'var(--color-secondary)', letterSpacing: '.04rem', flexShrink: 0 }}>
          ★ {formatMaterialName(motherlode)}
        </div>
      )}
      <div className='mining-panel__pie-area'>
        <PieDonut materials={materials} />
        <div className='mining-panel__pie-legend'>
          {materials.map((m, i) => (
            <div key={i} className='mining-panel__pie-legend-row'>
              <span className='mining-panel__pie-dot' style={{ background: m.isTarget ? 'var(--color-secondary)' : 'rgba(250,150,0,0.5)' }} />
              <span className={`mining-panel__pie-legend-name ${m.isTarget ? 'text-secondary' : ''}`} style={{ opacity: m.isTarget ? 1 : 0.6 }}>
                {formatMaterialName(m.name)}
              </span>
              <span className={`mining-panel__pie-legend-pct ${m.isTarget ? 'text-secondary' : ''}`} style={{ opacity: m.isTarget ? 1 : 0.6 }}>
                {m.pct.toFixed(1)}%
              </span>
            </div>
          ))}
          {rock > 2 && (
            <div className='mining-panel__pie-legend-row'>
              <span className='mining-panel__pie-dot' style={{ background: 'rgba(120,100,100,0.4)' }} />
              <span style={{ opacity: 0.3, fontSize: '.95rem' }}>ROCK</span>
              <span style={{ opacity: 0.3, fontSize: '.95rem' }}>&nbsp;&nbsp;{rock.toFixed(1)}%</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function MiningProspectorHistory ({ history, isOffline, notchCorner = null }) {
  const notchClass = notchCorner === 'bottom-right' ? 'mining-panel__prospector-card--notch-br'
    : notchCorner === 'top-right' ? 'mining-panel__prospector-card--notch-tr'
    : ''

  return (
    <div className={`mining-panel__prospector-card ${notchClass}`}>
      <div className='mining-panel__prospector-title'>
        {isOffline ? <span style={{ color: 'rgba(200,80,80,.6)' }}>OFFLINE</span> : 'PROSPECT HISTORY'}
      </div>
      {history.length === 0
        ? <div className='mining-panel__prospector-empty mining-panel__prospector-empty--ready'>—</div>
        : (
          <div className='mining-panel__history-table'>
            {history.map((r, i) => {
              const topMats = r.materials
                .filter(m => m.pct >= MIN_PERCENT)
                .sort((a, b) => (b.isTarget ? 1 : 0) - (a.isTarget ? 1 : 0) || b.pct - a.pct)
                .slice(0, 4)
              const contentLabel = CONTENT_LEVEL_LABELS[r.contentLevel] ?? r.contentLevel
              return (
                <div key={i} className={`mining-panel__history-row ${r.isGood ? 'mining-panel__history-row--good' : ''}`}>
                  <span className='mining-panel__history-time'>{formatTimeAgo(r.timestamp)}</span>
                  <span className={`mining-panel__history-level mining-panel__history-level--${r.contentLevel}`}>{contentLabel[0]}</span>
                  <span className='mining-panel__history-mats'>
                    {topMats.length > 0
                      ? topMats.map((m, j) => (
                        <span key={j} className={m.isTarget ? 'text-secondary' : ''} style={{ opacity: m.isTarget ? 1 : 0.55 }}>
                          {MINING_ABBREV[(m.name ?? '').toLowerCase().replace(/\s+/g, '')] ?? m.name?.slice(0, 2).toUpperCase()}{' '}
                          <span style={{ opacity: 0.7 }}>{m.pct.toFixed(0)}%</span>
                          {j < topMats.length - 1 ? '\u2002' : ''}
                        </span>
                      ))
                      : <span style={{ opacity: 0.3 }}>—</span>
                    }
                  </span>
                </div>
              )
            })}
          </div>
        )
      }
    </div>
  )
}

function MiningSessionCard ({ session, refined, limpets, estimatedProfit, className }) {
  const { totalRocks = 0 } = session ?? {}
  const { totalTons = 0, ratePerHour = 0 } = refined ?? {}
  const { used: limpetsUsed = 0 } = limpets ?? {}

  const profitDisplay = estimatedProfit > 0
    ? estimatedProfit >= 1e9
      ? `${(estimatedProfit / 1e9).toFixed(1)}B CR`
      : `${(estimatedProfit / 1e6).toFixed(1)}M CR`
    : null

  return (
    <div className={`mining-panel__info-card${className ? ' ' + className : ''}`}>
      <div className='mining-panel__section-title'>Statistics</div>
      <div className='mining-panel__stat-row'>
        <span className='mining-panel__stat-label'>Scanned</span>
        <span className='mining-panel__stat-value'>{totalRocks}</span>
      </div>
      <div className='mining-panel__stat-row'>
        <span className='mining-panel__stat-label'>Refined</span>
        <span className='mining-panel__stat-value'>
          {totalTons} t{ratePerHour > 0 && <span style={{ opacity: 0.6 }}> · {ratePerHour} t/hr</span>}
        </span>
      </div>
      <div className='mining-panel__stat-row'>
        <span className='mining-panel__stat-label'>Limpets Used</span>
        <span className='mining-panel__stat-value'>{limpetsUsed}</span>
      </div>
      <div className='mining-panel__stat-row'>
        <span className='mining-panel__stat-label'>Est. Profit</span>
        <span className='mining-panel__stat-value mining-panel__stat-value--highlight'>
          {profitDisplay ?? <span style={{ opacity: 0.3 }}>—</span>}
        </span>
      </div>
    </div>
  )
}

function MiningRefinedCard ({ refined, className }) {
  const { items: refinedItems = [] } = refined ?? {}
  return (
    <div className={`mining-panel__info-card${className ? ' ' + className : ''}`}>
      <div className='mining-panel__section-title'>Refined</div>
      {refinedItems.length > 0
        ? refinedItems.map((item, i) => (
          <div key={i} className='mining-panel__stat-row'>
            <span className='mining-panel__stat-label' style={{ textTransform: 'none', letterSpacing: 0 }}>
              {item.key}
            </span>
            <span className='mining-panel__stat-value mining-panel__stat-value--highlight'>{item.count} t</span>
          </div>
        ))
        : <div className='mining-panel__stat-placeholder'>— none yet —</div>
      }
    </div>
  )
}

export default function MiningDashboard ({ miningStatus, scaledWrapper, scaledContent }) {
  const isOffline = !miningStatus
  const { cargo, limpets, activeProspectors, session, refined, weaponPips, estimatedProfit } = miningStatus ?? {}

  return (
    <div ref={scaledWrapper} style={{ position: 'absolute', top: 0, bottom: 0, left: 0, right: 0, overflow: 'hidden' }}>
      <div ref={scaledContent} className='mining-panel__dashboard' style={isOffline ? { opacity: 1 } : undefined}>

        {/* Left column — both prospectors + laser, spans all 3 grid rows */}
        <div className='mining-panel__prosp-col'>
          <div className='mining-panel__prospector-wrap mining-panel__prospector-wrap--notch-br'>
            <MiningRecentProspect
              result={activeProspectors?.[0] ?? null}
              isOffline={isOffline}
              notchCorner='bottom-right'
            />
            <svg className='mining-panel__prospector-arc' viewBox='0 0 32 24.75' preserveAspectRatio='none'>
              <path d='M 32 0 L 32 19.06 A 12 12 0 0 0 29.21 24.75 L 0 24.75' />
            </svg>
          </div>
          <MiningLaserPower weaponPips={weaponPips} />
          <div className='mining-panel__prospector-wrap mining-panel__prospector-wrap--notch-tr'>
            <MiningProspectorHistory
              history={session?.recentResults ?? []}
              isOffline={isOffline}
              notchCorner='top-right'
            />
            <svg className='mining-panel__prospector-arc' viewBox='0 0 32 24.75' preserveAspectRatio='none'>
              <path d='M 0 0 L 29.21 0 A 12 12 0 0 0 32 5.69 L 32 24.75' />
            </svg>
          </div>
        </div>

        {/* Config — row 1 centre */}
        <div className='mining-panel__info-card-wrap mining-panel__info-card-wrap--config'>
          <div className='mining-panel__info-card mining-panel__info-card--notch-bottom'>
            <div className='mining-panel__section-title'>Configuration</div>
            <div className='mining-panel__stat-placeholder'>— buttons here —</div>
          </div>
          <svg className='mining-panel__info-card-arc' viewBox='0 0 16 18' preserveAspectRatio='none'>
            <path d='M 0 18 A 12 12 0 0 1 16 18' />
          </svg>
        </div>

        {/* Row 2 — circle only */}
        <div className='mining-panel__circle-wrap'>
          <div className='mining-panel__circle-outer'>
            <div className='mining-panel__circle-inner'>
              <img className='mining-panel__circle-icon' src='/icons/mining.svg' alt='' />
            </div>
          </div>
        </div>

        {/* Target — row 3 centre */}
        <div className='mining-panel__info-card-wrap mining-panel__info-card-wrap--target'>
          <div className='mining-panel__info-card mining-panel__info-card--notch-top'>
            <div className='mining-panel__section-title'>Target</div>
            <div className='mining-panel__stat-placeholder'>— target info —</div>
          </div>
          <svg className='mining-panel__info-card-arc' viewBox='0 0 16 18' preserveAspectRatio='none'>
            <path d='M 0 0 A 12 12 0 0 0 16 0' />
          </svg>
        </div>

        {/* Right column — stats + refined, spans all 3 grid rows */}
        <div className='mining-panel__right-col'>
          <div className='mining-panel__info-card-wrap'>
            <MiningSessionCard
              session={session}
              refined={refined}
              limpets={limpets}
              estimatedProfit={estimatedProfit}
              className='mining-panel__info-card--notch-bl'
            />
            <svg className='mining-panel__info-card-arc' viewBox='0 0 18.5 26.5' preserveAspectRatio='none'>
              <path d='M 0 0 L 0 19.063 A 12 12 0 0 1 2.989 26.5 L 18.5 26.5' />
            </svg>
          </div>
          <div className='mining-panel__info-card-wrap'>
            <MiningRefinedCard
              refined={refined}
              className='mining-panel__info-card--notch-tl'
            />
            <svg className='mining-panel__info-card-arc' viewBox='0 0 18.5 26.5' preserveAspectRatio='none'>
              <path d='M 18.5 0 L 2.989 0 A 12 12 0 0 1 0 7.437 L 0 26.5' />
            </svg>
          </div>
        </div>

        {/* Cargo bay + limpets — far right, span all rows via grid-area */}
        <MiningCargoBay cargo={cargo} />
        <MiningLimpetColumn limpets={limpets} cargo={cargo} />
      </div>
    </div>
  )
}
