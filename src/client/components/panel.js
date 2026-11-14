import PanelNavigation from 'components/panel-navigation'

export default function Panel ({ children, layout = 'full-width', scrollable = false, navigation, search, exit, style = {}, className = '', temporaryLabels = false }) {
  return (
    <div
      className={`layout__${layout} ${scrollable ? 'scrollable' : ''} ${(navigation && navigation.length > 0) ? 'layout__panel--secondary-navigation' : ''} ${className}`}
      style={{ ...style }}
    >
      {navigation && navigation.length > 0 &&
        <PanelNavigation temporaryLabels={temporaryLabels} items={navigation} search={search} exit={exit} />}
      {children}
    </div>
  )
}
