# Onboarding: questions for legal and compliance

Status: waiting for answers. Step 7 of the identity plan (onboarding) cannot
be built until these are settled, because they decide which fields and
documents we collect, how long we keep them and who may see them.

## What we plan to collect (proposal)

| Area | Fields |
|---|---|
| Business | Legal name, trading name, legal form, registration number, tax number, date of incorporation, sector, website, registered address, country |
| Activity | What is sold, expected monthly volume and average payment, countries served |
| People | Owners above a threshold and directors: full name, date of birth, nationality, role, ownership share, ID number |
| Contact | Business email and phone, a contact person |
| Payout | Where settlements go: mobile money number or bank account, in the business's name |
| Documents | Registration certificate, tax certificate, ID of each owner or director, proof of address, signed terms |

Documents are stored encrypted in object storage. Only metadata is in the
database. Staff see personal data masked, and revealing it is a logged action
with a reason.

## Questions

1. **Regulation.** Which authority and rules govern a payment aggregator in
   Gabon and the CEMAC zone? Do we need our own licence, or can we operate
   under a licensed partner? What does each option require before we go live?
2. **KYC content.** Which identity and business documents are mandatory for a
   merchant? Do requirements differ for individuals, sole traders and
   companies?
3. **Beneficial owners.** What ownership threshold makes a person a
   beneficial owner we must identify? Must we screen directors too?
4. **Screening.** Are sanctions, politically exposed person and adverse-media
   checks required? Which lists, how often, and who may provide the service?
5. **Risk and limits.** Are there mandatory transaction or monthly limits by
   merchant category? Which activities are prohibited or restricted?
6. **Retention.** How long must we keep KYC data and records after a merchant
   leaves? When must we delete it?
7. **Data location.** May personal data and documents be stored outside
   Gabon, for example in an Alibaba Cloud region elsewhere? Is a local
   region or copy required?
8. **Data protection.** Which data protection law applies, and do we need to
   register or appoint a data protection officer? What must a merchant be told
   when they submit their data?
9. **Reporting.** Which suspicious activity or periodic reports must we file,
   to whom, and by when?
10. **Terms.** What must the merchant agreement contain, and in which
    languages? Who signs for us?

## What we do while we wait

We build the parts that do not depend on the answers: the application and
review workflow, encrypted document storage, and the staff review tools. The
field list and document types stay configuration, so an answer changes data,
not code.
