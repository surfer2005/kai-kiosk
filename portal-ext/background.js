// KAI-Kiosk „Zurück zu KAI" — Hintergrund-Dienst (Service-Worker).
// Einzige Aufgabe: auf Zuruf des Content-Scripts (ausgelöst von KAI bei Abmeldung bzw. über
// „Portal-Anmeldungen löschen") ALLE Browserdaten der Portal-Seiten löschen — Cookies, Local Storage,
// IndexedDB, Cache, Service-Worker —, aber die KAI-Herkunft ausnehmen, damit die KAI-Anmeldung und das
// Offline-Bordbuch (IndexedDB/Service-Worker der PWA) erhalten bleiben. So startet der nächste
// Schiffsführer bei den Portalen sauber, ohne dass KAI selbst zurückgesetzt wird.
'use strict'

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || msg.type !== 'wipe') return
  const excludeOrigins = Array.isArray(msg.keepOrigins) && msg.keepOrigins.length
    ? msg.keepOrigins.filter((o) => typeof o === 'string' && o)
    : ['http://127.0.0.1:8091']
  chrome.browsingData.remove(
    { excludeOrigins },
    { cookies: true, localStorage: true, indexedDB: true, cacheStorage: true, serviceWorkers: true, cache: true },
    () => { try { sendResponse({ ok: !chrome.runtime.lastError }) } catch (e) {} },
  )
  return true // asynchrone Antwort
})
