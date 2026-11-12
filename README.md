<picture>
  <source media="(prefers-color-scheme: dark)" srcset="resources/images/title-whitetext.png">
  <img alt="DAEDALUS Terminal" src="resources/images/title.png" height="100" width="auto"/>
</picture>

<br>

![GitHub Version](https://img.shields.io/github/v/release/Varzeki/daedalus?display_name=tag&include_prereleases&sort=semver&color=cf7500&style=for-the-badge)
![GitHub License](https://img.shields.io/github/license/Varzeki/daedalus?color=cf7500&style=for-the-badge)
![GitHub Downloads](https://img.shields.io/github/downloads/Varzeki/daedalus/total?color=cf7500&style=for-the-badge)

> **DAEDALUS Terminal** is a fork of [ICARUS Terminal](https://github.com/iaincollins/icarus) by Iain Collins.
> For the original project, visit [iaincollins/icarus](https://github.com/iaincollins/icarus).

## About DAEDALUS Terminal

**Go/Wails developer preview (G1A):** an offline Windows/Linux desktop foundation is being developed separately from the working legacy application. It embeds the current React interface, but game data, integrations and remote second-screen pairing are unimplemented. See the [preview build and verification runbook](docs/testing/g1a-windows-linux-smoke.md). This is not a replacement release.

_DAEDALUS Terminal is a free, immersive, context-sensitive companion app and second screen interface for [Elite Dangerous](https://www.elitedangerous.com/)._

You can run DAEDALUS Terminal in a native window, on multiple windows/displays, as an overlayed window on top of the game if playing with a VR headset or on an ultra-wide display, or connect remotely in a browser from another computer/tablet/phone/other device (e.g. Chromebook, Android Phone/Tablet, Amazon Fire Tablet). The UI is specifically designed with touch screen devices in mind and adapts the layout of panels to both landscape and portrait displays, both large and small.

## Features Added in This Fork

The following features are exclusive to DAEDALUS Terminal and are not present in the original ICARUS Terminal.

### Engineering System

A complete engineering workflow built into the terminal, based on EDOMH.

* Wishlist
* Smart Routing
* Engineer Status Tracking
* Trading Logic

### Exobiology Predictor

A full featured prediction system based on formulas from SRVSurvey, BioInsights, and Exploration Buddy.

* Probabilities in percent
* On-planet bio scanner to help locate and scan different colonies
* Optimised prediction based on existing community methods

### Exploration 

An exploration tool based on Exploration Buddy.

* Auto-tab switching based on ingame state
* Route analysis
* System body analysis and highlighting

### COVAS Verity Voiceover

An extension of the existing ingame Verity COVAS

* Original ingame voicelines play in harmony with extended voicelines, using a queue to prevent overlap
* Additional events trigger extended audio voicelines, such as low fuel, valuable bodies found, warnings, etc.
* Fully configurable

### Landing Pad Overlay

Visual overlays showing pad layouts for all major station types, based on the EDMC plugin LandingPad:

The overlay highlights your assigned pad based on the docking grant in your journal.

### Journal and Connectivity Improvements

* Incremental journal log reading with 250 ms polling — new journal entries are detected immediately without re-reading the whole file.
* EDSM integration with automatic retry on transient failures.
* Improved event parsing for exploration and biological scan events.

---

## Screenshots

<img alt="Navigation Map" src="resources/images/screenshots/nav-map.png" height="auto" width="98.5%"/>

<img alt="Navigation List" src="resources/images/screenshots/nav-list.png" height="auto" width="49%"/> <img alt="Ship Status" src="resources/images/screenshots/ship-status.png" height="auto" width="49%"/>
<img alt="Exploration System" src="resources/images/screenshots/exploration-system.png" height="auto" width="49%"/> <img alt="Exploration Route" src="resources/images/screenshots/exploration-route.png" height="auto" width="49%"/>
<img alt="Blueprint" src="resources/images/screenshots/blueprint.png" height="auto" width="49%"/> <img alt="Navigation Route" src="resources/images/screenshots/nav-route.png" height="auto" width="49%"/>
<img alt="Log" src="resources/images/screenshots/log.png" height="auto" width="49%"/> <img alt="Materials" src="resources/images/screenshots/materials.png" height="auto" width="49%"/>

## Requirements

The self-contained installer is around 20 MB and has no dependencies. If you are running an older but supported release of Windows, any missing dependencies will be automatically installed.

* Windows 10 or newer required.
* No dependencies are required to install the application.
* No manual configuration or setup is required — it will automatically find your game data.
* No additional disk space is required to store game data. Recent game data is loaded into memory when launched and streamed in real time when the game is active; it is not persisted or cached to disk.

### Notes

* The application will run against the latest version of Elite Dangerous (Odyssey) and older releases (e.g. Horizons), but it is currently explicitly designed for use with the latest versions. Changes to the game API may impact functionality when using DAEDALUS Terminal with older versions of the game.
* The local web interface is enabled while the application is running. Remote displays require explicitly configured HTTPS and pairing from the Windows PC. See [secure browser setup and acceptance tests](docs/testing/phase-1a-windows-smoke.md).

The web interface relies on advanced browser functionality for rendering and works best on native Google Chrome browsers (e.g. Google Chrome for Windows, Mac, ChromeOS, Android). Other browsers (e.g. Safari, Firefox, Chromium) may use fallback rendering and/or lack required features for full compatibility / optimal user experience.

## Developer Documentation

For developer documentation see [BUILD.md](BUILD.md).

Regression checks: `npm test`, `npm run lint:security`, and `npm run build:client`.
The security tests use synthetic data and block external HTTP/HTTPS/fetch traffic.
Global legacy lint remains advisory while all changed security files have a
blocking lint gate. See the [Phase 1B Inara investigation](docs/handoffs/phase-1b-inara-scope.md)
for confirmed integration defects that remain deferred.

### Developer Quickstart

If you are running on Linux and/or looking for quick instructions on how to run from source, if you have [Node.js](https://nodejs.org/en/) this is what you need to do to download and install DAEDALUS Terminal:

    git clone git@github.com:YOUR_USERNAME/daedalus.git
    cd daedalus
    npm install

Next, run `cp .env-example .env` to create an `.env` file and edit it to change the `LOG_DIR` option to point to the location of your Elite Dangerous log files:

    # LOG_DIR can be used to tell the Service where to look for game logs
    # This option can be used in development and at runtime
    LOG_DIR=path/to/logs

With that done, anytime you want to start DAEDALUS Terminal, all you need to do is run:

    npm start

This will run in debug mode which is not quite the same as a production build (it's not as optimised) but should work just fine.

### Building a Standalone Binary

The quickest way to produce a runnable binary (no installer needed):

    npm install
    npm run build:standalone

This outputs a standalone executable in `build/`. See [BUILD.md](BUILD.md) for full build documentation including the Windows installer build.

## Legal

DAEDALUS Terminal is free, open-source software. The original ICARUS Terminal was released under the ISC License by Iain Collins. This fork is licensed under the [GNU General Public License v3.0](LICENSE) due to incorporation of code derived from GPL-3.0 licensed projects.

DAEDALUS Terminal does not record Personally Identifiable Information (PII). DAEDALUS Terminal includes an integration with [EDSM](https://www.edsm.net). Data such as your current in-game location may be sent to EDSM in order to render stellar cartography information in the interface. DAEDALUS Terminal does not expose or send information about you or your in-game character (e.g. your name, user name, commander name, or ship name), but any requests made to a third party will include your IP address.

Elite Dangerous is copyright Frontier Developments plc. This software is not endorsed by nor reflects the views or opinions of Frontier Developments and no employee of Frontier Developments was involved in the making of it.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.

## Credits

_DAEDALUS Terminal would not be possible without the original [ICARUS Terminal](https://github.com/iaincollins/icarus) by Iain Collins, and work from dozens of enthusiasts and hundreds of open source contributors._

* The original name ICARUS was suggested by [SpaceNinjaBear](https://www.reddit.com/user/SpaceNinjaBear) on Reddit.
* Loading animation by [James Panter](http://codepen.io/jpanter/pen/PWWQXK).
* Includes original icons, icons inspired by those found in Elite Dangerous, and icons based on those from [edassets.org](https://edassets.org).
* Uses stellar cartography data from the [EDSM API](https://www.edsm.net).
* Includes game data collated and provided by [EDCD](https://github.com/EDCD/FDevIDs).
* The [Jura font](https://fonts.google.com/specimen/Jura#glyphs) is included under the Open Font License.
* Thank you to [Serge Zaitsev](https://github.com/zserge) for his work on the WebView library.

### Fork Credits

This fork adds features built on the work of the following projects and communities:

* **[SrvSurvey](https://github.com/njthomson/SrvSurvey)** by njthomson — biological species prediction criteria, exploration value algorithms, and settlement template pad positions. Licensed under [GPL-3.0](https://github.com/njthomson/SrvSurvey/blob/main/LICENSE).
* **[elite-dangerous-odyssey-materials-helper](https://github.com/jixxed/ed-odyssey-materials-helper)** by jixxed — engineer portrait images used in the Engineer Cards page along with workflow layout and design used to create the engineering portion. Licensed under [MIT](https://github.com/jixxed/ed-odyssey-materials-helper/blob/master/LICENSE).
* **[LandingPad](https://github.com/bgol/LandingPad)** by bgol — reference for starport and fleet carrier pad layout templates used in the Landing Pad Overlay, as well as logic.
* **[Canonn Research Group](https://canonn.science/)** — species observation data, codex imagery, and community research used in the biological predictor.
* **[Spansh](https://spansh.co.uk/)** — system and body data API used for material trader routes and powerplay system data.
* **[Elite Observatory BioInsights](https://edjp.colacube.net/observatory)**  — methodology reference for bio signal analysis and species criteria.
* **[Elite Dangerous Exploration Buddy](https://www.panostrede.de/EDEB/)** — reference for exploration body value calculations.
* **[EDDB](https://eddb.io/)** — Elite Dangerous Database; a community resource for station, commodity, and engineer data that informed many design decisions in this project.
* **[INARA](https://inara.cz/)** — Elite Dangerous companion site providing commander profiles, engineering data, and community knowledge referenced during development.

Thank you to all those who have created and supported libraries on which this software depends and to Frontier Developments for supporting third party tools.


