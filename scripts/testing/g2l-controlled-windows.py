"""Bounded paired exact-commit investigation; no application/network operations.
Builds test binaries once, then fresh processes/profiles, AB/BA/AB for both workloads.
Use --prepare to compile only, then --run PATH once after other development checks.
"""
import argparse,datetime,hashlib,json,os,pathlib,platform,re,statistics,subprocess,time
ROOT=pathlib.Path(__file__).resolve().parents[2]
COMMITS={'g2k':'fd721266d93bf86976de029d5ce3e626e57a6786','g2l':'54a90b36ca8ee6ddfb09dfcd607ef967e6ea19f8'}
GO=ROOT/'temp/g1a-tools/go/bin/go.exe'
CC=pathlib.Path('C:/codex-daedalus-build-tools-20261115/mingw64/bin/gcc.exe')
ENV={**os.environ,'CC':str(CC),'CGO_ENABLED':'1','GOTOOLCHAIN':'local','PATH':str(CC.parent)+os.pathsep+os.environ['PATH']}
def call(args,cwd=ROOT,**kw):
 return subprocess.run([str(a) for a in args],cwd=cwd,env=ENV,text=True,encoding='utf8',errors='replace',check=True,creationflags=subprocess.CREATE_NO_WINDOW,**kw)
def prepare():
 out=ROOT/'temp'/('g2l-controlled-'+datetime.datetime.now().strftime('%Y%m%d-%H%M%S'));out.mkdir(parents=True)
 overlay=ROOT/'scripts/testing/fixtures/g2l-review-mixed_test.go'
 meta={'commits':COMMITS,'platform':platform.platform(),'machine':platform.machine(),'processor':platform.processor(),'go':call([GO,'version'],capture_output=True).stdout.strip(),'cc':call([CC,'--version'],capture_output=True).stdout.splitlines()[0],'overlaySHA256':hashlib.sha256(overlay.read_bytes()).hexdigest(),'rounds':3,'order':['g2k,g2l','g2l,g2k','g2k,g2l'],'workloads':{'material':'100000 MaterialCollected-dominated records, 2 commanders; original 1000 catch-up lifecycle','mixed':'10000 fleet/loadout/module/carrier/trade/mining/CG/crew records, 2 commanders; cold import only; Market markers have no historical sidecar attribution'}}
 for label,commit in COMMITS.items():
  folder=out/label;folder.mkdir();archive=out/(label+'.tar')
  call(['git','archive','--format=tar','--output='+str(archive),commit,'go']);call(['tar','-xf',archive,'-C',folder])
  (folder/'go/internal/gamestate/g2l_review_mixed_test.go').write_bytes(overlay.read_bytes())
  call([GO,'test','-c','-o',out/(label+'.test.exe'),'./internal/gamestate'],cwd=folder/'go',capture_output=True)
  call([out/(label+'.test.exe'),'-test.list=BenchmarkReviewFleetTradeCold'],capture_output=True)
 (out/'environment.json').write_text(json.dumps(meta,indent=2),encoding='utf8');print(out,flush=True)
 return out

def profile(out):
 out=pathlib.Path(out).resolve();assert out.is_relative_to((ROOT/'temp').resolve())
 assert (out/'results.json').exists(),'Run the bounded comparison before profiling'
 assert not (out/'profile-results.json').exists(),'Refusing repeated profiling'
 timings=out/'stage-profiles';timings.mkdir(exist_ok=True)
 for label in COMMITS:
  go=out/label/'go'
  hist=go/'internal/history'
  # Instrument only the disposable exact-source copies, never the application.
  helper='''package history
import("database/sql";"sync/atomic";"time")
var reviewCommitNS,reviewNormalizeNS,reviewOperationNS,reviewDurableNS,reviewCommitCalls,reviewOperationCalls atomic.Int64
func reviewDurableCommit(tx *sql.Tx)error{start:=time.Now();defer func(){reviewDurableNS.Add(time.Since(start).Nanoseconds())}();return tx.Commit()}
func ReviewTimings()map[string]float64{return map[string]float64{"store-commit":float64(reviewCommitNS.Load()),"normalize":float64(reviewNormalizeNS.Load()),"operations":float64(reviewOperationNS.Load()),"durable-sql-commit":float64(reviewDurableNS.Load())}}
func ReviewCounts()map[string]float64{return map[string]float64{"store-commits":float64(reviewCommitCalls.Load()),"operation-records":float64(reviewOperationCalls.Load())}}
'''
  (hist/'review_profile.go').write_text(helper,encoding='utf8')
  store=hist/'store.go';text=store.read_text(encoding='utf8');a=text.index('func (s *Store) Commit(');b=text.index('func (s *Store) Stats()',a);part=text[a:b];pos=part.index(' error {')+len(' error {');part=part[:pos]+'\nreviewStart:=time.Now();reviewCommitCalls.Add(1);defer func(){reviewCommitNS.Add(time.Since(reviewStart).Nanoseconds())}()\n'+part[pos:];part=part.replace('return tx.Commit()','return reviewDurableCommit(tx)');store.write_text(text[:a]+part+text[b:],encoding='utf8')
  normal=hist/'normalize.go';text=normal.read_text(encoding='utf8').replace('"strings"','"strings";"time"',1);a=text.index('func (s *Store) normalize(');pos=text.index(' error {',a)+len(' error {');text=text[:pos]+'\nreviewStart:=time.Now();defer func(){reviewNormalizeNS.Add(time.Since(reviewStart).Nanoseconds())}()\n'+text[pos:];normal.write_text(text,encoding='utf8')
  ops=hist/'operations.go'
  if ops.exists():
   text=ops.read_text(encoding='utf8').replace('"strings"','"strings";"time"',1);pos=text.index('\n\tif len(v) == 0 {');text=text[:pos]+'\nreviewStart:=time.Now();reviewOperationCalls.Add(1);defer func(){reviewOperationNS.Add(time.Since(reviewStart).Nanoseconds())}()\n'+text[pos:];ops.write_text(text,encoding='utf8')
  parser=go/'internal/journal/parser.go';text=parser.read_text(encoding='utf8').replace('"time"','"time";"sync/atomic"',1);pos=text.index('\n\tvar event Event');text=text[:pos]+'\nreviewStart:=time.Now();defer func(){reviewParserNS.Add(time.Since(reviewStart).Nanoseconds())}()\n'+text[pos:];text+='\nvar reviewParserNS atomic.Int64\nfunc ReviewParseNS()float64{return float64(reviewParserNS.Load())}\n';parser.write_text(text,encoding='utf8')
  shim='''package gamestate
import("os";"testing";"github.com/Jakoczig/daedalus/go/internal/history";"github.com/Jakoczig/daedalus/go/internal/journal")
func BenchmarkReviewProfile(b *testing.B){if os.Getenv("REVIEW_WORKLOAD")=="mixed"{BenchmarkReviewFleetTradeCold(b)}else{BenchmarkG2JLifecycle(b)};for k,v:=range history.ReviewTimings(){b.ReportMetric(v/1e6,k+"-ms")};for k,v:=range history.ReviewCounts(){b.ReportMetric(v,k)};b.ReportMetric(journal.ReviewParseNS()/1e6,"journal-decode-ms")}
'''
  (go/'internal/gamestate/review_profile_test.go').write_text(shim,encoding='utf8')
  call([GO,'test','-c','-o',timings/(label+'.test.exe'),'./internal/gamestate'],cwd=go,capture_output=True)
 results=[]
 for workload in ['material','mixed']:
  for label in COMMITS:
   ENV['REVIEW_WORKLOAD']=workload;prefix=f'{workload}-{label}';cpu=timings/(prefix+'.cpu.pprof')
   r=call([timings/(label+'.test.exe'),'-test.run=^$','-test.bench=^BenchmarkReviewProfile$','-test.benchtime=1x','-test.count=1','-test.timeout=5m','-test.cpuprofile='+str(cpu)],capture_output=True,timeout=360)
   (timings/(prefix+'.txt')).write_text(r.stdout+r.stderr,encoding='utf8')
   top=call([GO,'tool','pprof','-top','-nodecount=18',timings/(label+'.test.exe'),cpu],capture_output=True)
   (timings/(prefix+'-cpu.txt')).write_text(top.stdout+top.stderr,encoding='utf8')
   line=next(x for x in r.stdout.splitlines() if x.startswith('BenchmarkReviewProfile'))
   row={'workload':workload,'version':label,'stageMetrics':{name:float(value) for value,name in re.findall(r'([0-9.]+)\s+(store-commit-ms|normalize-ms|operations-ms|durable-sql-commit-ms|journal-decode-ms|store-commits|operation-records)',line)},'note':'Instrumented diagnostic, stage walls overlap. Material includes original warm/1000 catch-up lifecycle; mixed is cold only. Not an extra controlled timing sample.'};results.append(row);print(json.dumps(row),flush=True)
 (out/'profile-results.json').write_text(json.dumps(results,indent=2),encoding='utf8')


def run(out):
 out=pathlib.Path(out).resolve();assert out.is_relative_to((ROOT/'temp').resolve())
 assert not (out/'results.json').exists(),'Refusing accidental repeated investigation'
 results=[];profile=out/'profiles';profile.mkdir(exist_ok=True);ENV.update(TEMP=str(profile),TMP=str(profile))
 for round_no in range(3):
  order=['g2k','g2l'] if round_no%2==0 else ['g2l','g2k']
  for workload,benchmark in [('material','BenchmarkG2JLifecycle'),('mixed','BenchmarkReviewFleetTradeCold')]:
   for label in order:
    before=time.perf_counter();r=call([out/(label+'.test.exe'),'-test.run=^$','-test.bench=^'+benchmark+'$','-test.benchtime=1x','-test.count=1','-test.timeout=5m'],capture_output=True,timeout=360)
    wall=time.perf_counter()-before;filename=f'{round_no+1}-{workload}-{label}.txt';(out/filename).write_text(r.stdout+r.stderr,encoding='utf8')
    m=re.search(r'([0-9.]+)\s+cold-ms',r.stdout);assert m,r.stdout
    row={'round':round_no+1,'workload':workload,'commit':COMMITS[label],'version':label,'coldMS':float(m[1]),'wallSeconds':wall,'log':filename};results.append(row)
    (out/'progress.json').write_text(json.dumps(results,indent=2),encoding='utf8');print(json.dumps(row),flush=True)
 summary={}
 for workload in ['material','mixed']:
  summary[workload]={}
  for label in COMMITS:
   values=[r['coldMS'] for r in results if r['workload']==workload and r['version']==label]
   summary[workload][label]={'samplesMS':values,'medianMS':statistics.median(values),'minMS':min(values),'maxMS':max(values)}
  summary[workload]['medianRatioG2LOverG2K']=summary[workload]['g2l']['medianMS']/summary[workload]['g2k']['medianMS']
 report={'environment':json.loads((out/'environment.json').read_text(encoding='utf8')),'samples':results,'summary':summary}
 (out/'results.json').write_text(json.dumps(report,indent=2),encoding='utf8');print(json.dumps(summary),flush=True)
if __name__=='__main__':
 if os.name!='nt':raise SystemExit('This controlled investigation must run on Windows')
 p=argparse.ArgumentParser();p.add_argument('--prepare',action='store_true');p.add_argument('--run');p.add_argument('--profile');a=p.parse_args()
 if a.prepare:prepare()
 elif a.run:run(a.run)
 elif a.profile:profile(a.profile)
 else:p.error('choose --prepare or --run DIRECTORY')
