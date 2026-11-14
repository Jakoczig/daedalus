import GoNavigation from './go-navigation'
import GoSurveyExplorer from './go-survey-explorer'
export default function GoExplorationWorkspace (props) {
  const route = ['/nav/route', '/exploration/route'].includes(props.path)
  return (
    <>
      {route ? <GoNavigation {...props} enhanced /> : <GoSurveyExplorer {...props} biology={props.path === '/exploration/biologicals'} listOnly={props.path === '/nav/list'} />}
      {route && props.nativePreview && props.role === 'owner' && <GoSurveyExplorer {...props} />}
    </>
  )
}
