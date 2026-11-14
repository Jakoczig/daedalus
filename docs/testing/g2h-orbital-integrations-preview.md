# DAEDALUS G2H — EXPERIMENTAL, UNSIGNED ORBITAL / INTEGRATIONS PREVIEW

One combined G2H/G2I batch on G2G. Portable executable, no installer/Release.
Keep the original application intact. Close another Go preview first (exclusive profile lock).
Extract executable, README.md, SHA256SUMS. Windows: `Get-FileHash .\daedalus.exe -Algorithm SHA256`.
Linux: `sha256sum -c SHA256SUMS`, `chmod +x daedalus`, then `./daedalus`.
Windows 10/11 x64 needs WebView2 Evergreen. Linux x64 needs GTK4, WebKitGTK 6.0,
graphical session and session D-Bus (Ubuntu 24.04: libgtk-4-1 libwebkitgtk-6.0-4).
Linux API credentials additionally require libsecret-tools (`secret-tool`) and an unlocked
Secret Service. Absence never prevents the offline desktop; credentials fail closed.
Tray behavior needs a StatusNotifier host; fallback minimizes the launcher. Physical
Linux/Wayland/Proton/GPU/tray and actual game/laptop acceptance remain manual.

| Feature | Implementation / evidence status |
|---|---|
| Orbital Cartography / Catalogue | Local, original icons; horizontal stellar branches, vertical moons, explicit Null grouping, unresolved parents separate; full retained bodies, Fit/zoom; no duplicate strip; compact inspector and system-level stations. Synthetic browser tested. |
| Ship Statistics | Local journal jump/mass/fuel/cargo/hull/rebuy/loadout freshness. Custom ship name owner-only. Effective power uses explicit engineering or verified unengineered references; unsupported totals/flight curves remain UNAVAILABLE. |
| Galaxy | Local route framing, manual pan/zoom, Fit Route/Galaxy and separate search marker. Original illustrative texture, not measured density. Optional cached EDAstro GEC POI/nebula toggles. No licensed region/visit-density dataset packaged. |
| EDSM | Existing read-only map setting preserved. Separately consented account flight-log readback and journal travel/scan/loadout/material/rank upload adapter; offline contract-tested, credentials/live acceptance pending. |
| Inara | Profile read; supported travel, loadout/engineering, material inventory, pilot ranks/progress and witnessed Statistics credit updates transformed into official API events. Fixture-tested; application whitelisting absent, production adapter gated. Not a general market/ship-calculation API. |
| EDAstro | Explicit system/GEC reads; selected ScanOrganic / sanitized CarrierStats / CarrierJumpRequest submissions. Independent sensitive carrier categories. Fixture-tested, no live submission performed. GEC API data are attributed, no proprietary imagery/dumps packaged. |
| EDDN | Schema-validated journal/1 FSDJump, Location, CarrierJump, Scan, SAASignalsFound publication; opt-in travel/exploration categories. Fixture-tested, no live publication. No permanent subscription or historical search. Market/outfitting/shipyard schemas are not implemented. |
| Frontier CAPI | Gated OAuth Authorization Code PKCE/system browser, one-use expiring state, exact loopback callback, secure token refresh and throttled profile/market/shipyard/fleetcarrier reference reads. Fixtures pass; developer approval/client ID, registered callback and live acceptance absent. No game writes. |

## Provider controls and privacy
Launcher → Settings → Data Providers. Existing EDSM map read consent remains separate.
Every NEW read/upload starts OFF. Choose categories, acknowledge disclosure and Save consent;
this binds uploads to the witnessed active commander. Save resumes a paused configuration.
Pause/disconnect and category removal cancel reserved/in-flight requests and clear queues.
Remove stored credentials deletes that provider's saved secret. No password is requested for Frontier.
Windows DPAPI / Linux Secret Service-backed encrypted vault protect keys/tokens; no plaintext fallback.
Viewer devices cannot access credentials, private snapshots/names, initiate network traffic or host actions.
Public map layers are typed cached points only. Account snapshots supplement matching commander
views and never overwrite journal facts. Provider sees IP and submitted fields, including disclosed
galactic locations. Carrier destinations can be sensitive. Avoid reporting the same data via EDMC/EDD.

One shared journal pipeline; per-provider event-driven workers park when idle. Queues cap at 64 events /
2 MiB each; payload 256 KiB, response 2 MiB. EDSM/Inara batch up to 16 events. Read requests at most
one/minute/provider; upload intervals are shown in settings, explicit refusal retries back off with jitter.
No import/replay/startup resubmission, unknown/Legacy/Beta version or stale events. Durable dedupe
covers fresh live records over restarts (10-minute / 4096 hash budget, fail closed when exhausted).
Timeout delivery is ambiguous: no automatic duplicate retry; an event can be lost. No durable raw
journal upload spool is created. No end-to-end exactly-once delivery claim.

Inara/Frontier are intentionally gated in these builds. After provider approval a maintainer must supply
the approved application name/client ID at build time (`providers.InaraApplicationName` /
`providers.FrontierClientID` linker variables). Frontier requires the exact existing loopback callback
`http://127.0.0.1:<current-service-port>/owner/frontier-callback` registered for that client; live callback
registration and production setup remain pending. No extra callback listener is opened.
API success in fixtures does not certify a real account, application approval or public publication.

Protocols: [EDSM journal](https://www.edsm.net/en/api-journal-v1),
[EDSM logs](https://www.edsm.net/es/api-logs-v1), [Inara](https://inara.cz/elite/inara-api-docs/),
[EDAstro read](https://edastro.com/api-details.html),
[EDAstro submission](https://edastro.com/api-info.html),
[EDMC EDAstro protocol reference](https://github.com/EDCD/EDMarketConnector/blob/main/plugins/edastro_core.py),
[EDDN live schemas](https://github.com/EDCD/EDDN/tree/live),
[Frontier OAuth/CAPI notes](https://github.com/EDCD/FDevIDs/tree/master/Frontier%20API).
No third-party implementation code or EDAstro imagery was copied; EDDN schema is embedded with license below.

## One gameplay + consent + laptop session
1. Verify default offline launcher, native dashboard close/tray/explicit Quit and paired laptop recovery.
2. Inspect a populated and a binary/multi-star system: planet/moon/barycentre links, unresolved parents,
   Fit/zoom, catalogue and compact inspector; compare Ship Statistics with game Loadout/Status.
3. Plot nearby/distant route; check all three Galaxy projections, Fit/manual pan/zoom and cached system view.
4. Explicitly enable only a desired provider read; fetch and inspect provenance/freshness/GEC layers. Confirm
   uploads stay off and viewer requests are denied. Check Inara/Frontier approval gates instead of fake success.
5. Only if intentionally testing real publication: review exact categories/account, consent once, observe counts
   and traffic, pause/category-off/disconnect/remove key, restart and verify no replay; switch commanders,
   cancel FSD/manual override/reconnect/revoke laptop. Monitor long-idle native CPU/RAM and Linux tray.

## EDDN schema license
BSD 3-Clause License

Copyright (c) 2018, EDDN
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of the copyright holder nor the names of its
  contributors may be used to endorse or promote products derived from
  this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## JSON Schema validator license (v5.3.1)

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.