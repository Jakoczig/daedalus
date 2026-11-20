const credits = [
  ['Elite Dangerous', 'Frontier Developments', 'Game journal and consented account reference.', 'https://www.frontier.co.uk/'],
  ['EDSM', 'EDSM community', 'Community system references and atlas source data.', 'https://www.edsm.net/'],
  ['Galaxy Painter', 'Jakoczig', 'Signed offline galaxy density tiles.', 'https://github.com/Jakoczig/Eridion-Galaxy-Painter'],
  ['Spansh', 'Spansh', 'Manual route planning and reference lookups.', 'https://spansh.co.uk/'],
  ['EDAstro', 'EDAstro community', 'Optional galaxy community layers.', 'https://edastro.com/'],
  ['FDevIDs', 'EDCD contributors', 'Game identifier reference data.', 'https://github.com/EDCD/FDevIDs'],
  ['EDDN', 'EDCD contributors', 'Separately consented community event distribution.', 'https://github.com/EDCD/EDDN'],
  ['EliteBGS', 'EliteBGS contributors', 'Optional public faction, system and tick references.', 'https://elitebgs.app/'],
  ['Canonn', 'Canonn community', 'Optional public science-site references.', 'https://docs.canonn.tech/home/about.html'],
  ['Raven Colonial', 'Raven Colonial maintainers', 'Optional public colonisation project references.', 'https://ravencolonial100-awcbdvabgze4c5cq.canadacentral-01.azurewebsites.net/About'],
  ['Inara', 'Inara maintainers', 'Approval-gated adapter; live account functionality remains unavailable.', 'https://inara.cz/elite/inara-api-docs/'],
  ['BGS-Tally', 'tezw21 / aussig', 'Activity-accounting workflow reference; MIT.', 'https://github.com/aussig/BGS-Tally'],
  ['BioScan', 'Jeremy Rimpo', 'Licensed species rules and sampling workflow; GPL.', 'https://github.com/Silarn/EDMC-BioScan'],
  ['ExploData', 'Silarn', 'Licensed genus reference distances; GPL.', 'https://github.com/Silarn/EDMC-ExploData'],
  ['Neutron Dancer', 'dwomble', 'CSV route workflow reference; MIT.', 'https://github.com/dwomble/EDMC-NeutronDancer'],
  ['Wails', 'Wails contributors', 'Native Windows and Linux desktop shell.', 'https://wails.io/'],
  ['Go', 'The Go authors', 'Native journal, storage, cryptography and network services.', 'https://go.dev/'],
  ['Next.js', 'Vercel and contributors', 'Builds the embedded static React interface.', 'https://nextjs.org/'],
  ['React', 'Meta and contributors', 'Terminal user interface.', 'https://react.dev/'],
  ['SQLite', 'SQLite authors', 'Local transactional gameplay history.', 'https://sqlite.org/']
]
export default function GoCredits () {
  return <section aria-label='Credits'><p>Sources and components used by ERIDION. No endorsement is implied; full license notices remain distributed with the source.</p>{credits.map(([name, creator, purpose, url]) => <article key={name}><h3><a href={url} target='_blank' rel='noreferrer'>{name}</a> — {creator}</h3><p>{purpose}</p></article>)}</section>
}
