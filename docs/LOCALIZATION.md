# Interface languages and in-app help

English is the default and fallback language. Choose **Language / भाषा / ભાષા** in the sidebar or login screen to switch to Hindi or Gujarati. The choice is stored under `ledgerleaf.language` in this browser's local storage, not in the business database. Other browsers or devices have their own preference. All translations and help guides are bundled for offline use; no translation service or network font is used.

## Current scope

- The in-app Help section has 11 guides in English, Hindi and Gujarati, covering setup, stock/units, billing/POS, payments/returns, tax/reports, backup/restore, imports, permissions/language, dialogs, accounting and goods workflows.
- Navigation and approximately 200 common interface labels/actions have Hindi and Gujarati translations.
- Some specialized descriptions, dynamic status text and server validation errors still fall back to English. PDFs and other exports are not translated by this setting. This is initial interface localization, not full application or statutory-document translation.
- Names, SKUs, account codes, API enum values, saved records and money/quantity formats stay unchanged. Users can enter their own supported Unicode names as before.
- Help shortcuts respect page visibility for the signed-in role. Help does not grant permission to perform restricted actions.

## Implementation

`frontend/src/i18n.ts` stores exact English UI strings with `[Hindi, Gujarati]` translations. `t(text)` returns the translated copy or unchanged English when absent. `useLanguage()` subscribes to changes and sets the HTML language; the root app subscribes so existing screens rerender without reloading or clearing form data. Separate React roots should subscribe themselves. Storage exceptions fall back to an in-memory selection.

Use `t()` only for interface copy. Never translate data from the database or values submitted to APIs. When translating option text, set an explicit unchanged `value` on the option so its label does not become a backend code. Do not translate field `name`, route keys, import headers, identifiers or amounts.

`frontend/src/help_content.ts` contains topic IDs, related page keys, titles, steps and optional notes as `[English, Hindi, Gujarati]` arrays. `help.tsx` renders and searches all three languages, offers permitted page shortcuts, and provides the language selector. The header Help action selects a related topic when one exists.

System fonts provide shaping and glyphs: Noto Sans Devanagari/Gujarati where installed, or Windows Nirmala UI/Mangal/Shruti fallbacks. The Linux screenshots were reviewed with Noto fonts. Final Windows language/font rendering and native-speaker editorial review remain outstanding.
