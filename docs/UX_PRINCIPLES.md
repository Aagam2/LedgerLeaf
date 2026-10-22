# Simplicity and familiar financial workflows

Simplicity and a familiar billing/accounting feel are core product requirements. New features should follow these conventions and improve the existing workflow before adding another screen or control.

- Keep everyday tasks immediately visible: billing, POS, parties, products, expenses and reports. Group less frequent document, stock, accounting and setup tools by purpose. Hide empty groups for restricted roles and clearly mark the current page.
- Use familiar business language: sales bill, purchase bill, customer, supplier, amount, due date and payment. Display readable labels while retaining stable API/database codes internally.
- Start each screen with its purpose and a clear primary action. Keep technical explanations in contextual help or beside the advanced option they explain.
- Show essential fields first. Put optional tax, delivery, discounts and configuration behind clearly named expandable sections. Do not conceal information required to make a financial decision.
- Keep list, search, filters and paging visible under an entry dialog. Preserve entered values when validation fails. Close the form and refresh the affected list after a successful save.
- Make sale and purchase entry easy to distinguish. Start a purchase with its supplier field and purchase prices; keep the draft → review → finalize sequence consistent.
- Use one consistent dialog, button, form, table and selection pattern. Provide readable empty states, visible keyboard focus, mobile scrolling and clear Cancel/Close behavior.
- Confirm irreversible financial actions with their concrete effect. Cancel must leave saved records unchanged. Simplifying the interface must preserve permissions, posting validation and accounting safeguards.
- Translate new common labels into the supported UI languages. Stored names, amounts, tax classifications and document identifiers retain their original meaning.

Before considering a usability change complete, check the ordinary path, a rejected submission, cancellation, a restricted role and a narrow screen where relevant. Interface checks with an in-memory API do not establish database or financial-posting correctness. Record that distinction in the test report.

## Current pass — 2026-10-22

The menu now keeps everyday tasks visible and groups other tools into expandable sections. Business details and first-use instructions are collapsed on the overview. Quick actions prioritize billing, POS, expenses, parties, products and reports. Billing offers direct purchase entry, a stronger primary create action, readable sale/purchase labels and concise introductory guidance. Tax guidance remains with the tax options; return/refund guidance remains inside the return dialog.

Further work should apply these same conventions to specialist screens, remaining internal-code labels and full translation. This pass establishes a consistent direction; it does not claim a complete usability or accessibility audit.
