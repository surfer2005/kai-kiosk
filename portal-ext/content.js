// KAI-Kiosk „Zurück zu KAI" — Content-Script.
// Läuft auf JEDER Seite im Kiosk-Chromium:
//  - setzt auf <html> die Marke data-kai-portal-ext (damit KAI erkennt, dass der Rückkehr-Knopf da ist
//    und die Portal-Kacheln im Schiffsterminal aktiv schalten darf — ohne die Erweiterung wären sie
//    gesperrt, s. PortaleView.vue);
//  - auf FREMDEN Seiten (Portalen) blendet es unten links einen dauerhaften „⟵ KAI"-Knopf ein, der
//    über die lokale Boot-Adresse (http://127.0.0.1:8091/boot → leitet auf die KAI-Adresse) zurückführt
//    — Touch-tauglich, keine Tastatur nötig;
//  - auf der KAI-Seite selbst zeigt es KEINEN Knopf, sondern nimmt die Abmelde-/Aufräum-Nachricht von
//    KAI entgegen (window.postMessage) und reicht sie an den Hintergrund-Dienst weiter, der dann alle
//    Portal-Anmeldungen löscht (außer der KAI-Herkunft).
'use strict'

const EXT_VERSION = '1.0.0'
const BOOT_URL = 'http://127.0.0.1:8091/boot'
const DAEMON_HOST = '127.0.0.1'

// Marke so früh wie möglich setzen (documentElement existiert schon bei document_start).
try { document.documentElement.setAttribute('data-kai-portal-ext', EXT_VERSION) } catch (e) {}

// KAI-Seite erkennen: KAIs index.html trägt <meta name="kai-app" content="1">. Zusätzlich gilt die
// lokale Boot-/Konfig-Seite (127.0.0.1) als „KAI-Seite" (kein Rückkehr-Knopf dort).
function istKaiSeite() {
  if (location.hostname === DAEMON_HOST) return true
  return !!document.querySelector('meta[name="kai-app"]')
}

// Rückkehr-Knopf auf fremder Seite einblenden.
function zeigeZurueckKnopf() {
  if (document.getElementById('kai-back-btn')) return
  const btn = document.createElement('button')
  btn.id = 'kai-back-btn'
  btn.type = 'button'
  btn.textContent = '⟵ KAI'
  btn.setAttribute('aria-label', 'Zurück zu KAI')
  Object.assign(btn.style, {
    position: 'fixed', left: '16px', bottom: '16px', zIndex: '2147483647',
    display: 'inline-flex', alignItems: 'center', gap: '7px',
    padding: '12px 20px', border: 'none', borderRadius: '999px',
    background: '#1f6e5c', color: '#fff', font: '700 16px/1 system-ui, sans-serif',
    cursor: 'pointer', boxShadow: '0 6px 18px rgba(0,0,0,.30)',
    WebkitTapHighlightColor: 'transparent', touchAction: 'manipulation',
  })
  // Zurück NICHT auf die Standard-Ansicht (Bordbuch), sondern auf die Portale-Seite: der Boot-Weg
  // reicht `view=portale` an KAI durch (Nutzerwunsch — nach dem Schließen einer Portal-Seite wieder
  // bei den Portal-Kacheln landen, nicht im Bordbuch).
  btn.addEventListener('click', () => { window.location.assign(BOOT_URL + '?view=portale') })
  document.body.appendChild(btn)
}

// Abmelde-/Aufräum-Nachricht von der KAI-Seite entgegennehmen und an den Hintergrund-Dienst reichen.
function installiereWipeRelais() {
  window.addEventListener('message', (ev) => {
    const d = ev && ev.data
    if (!d || d.source !== 'kai-portal' || d.type !== 'wipe') return
    // Herkunft, die NICHT gelöscht werden soll: die KAI-Seite selbst + der lokale Dienst.
    const keep = [location.origin, 'http://127.0.0.1:8091']
    if (Array.isArray(d.keepOrigins)) keep.push(...d.keepOrigins)
    try { chrome.runtime.sendMessage({ type: 'wipe', keepOrigins: keep }) } catch (e) {}
  })
}

function start() {
  if (istKaiSeite()) {
    installiereWipeRelais()
  } else {
    if (document.body) zeigeZurueckKnopf()
    else document.addEventListener('DOMContentLoaded', zeigeZurueckKnopf, { once: true })
  }
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', start, { once: true })
} else {
  start()
}
