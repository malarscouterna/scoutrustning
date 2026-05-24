# Changelog

## [0.9.4](https://github.com/malarscouterna/scoutrustning/compare/v0.9.3...v0.9.4) (2026-05-24)


### Bug Fixes

* **web:** show persona switcher for unregistered demo users, fix login links ([6eb2073](https://github.com/malarscouterna/scoutrustning/commit/6eb20738806d5aaae9cf4e2f664192411deecf4e))

## [0.9.3](https://github.com/malarscouterna/scoutrustning/compare/v0.9.2...v0.9.3) (2026-05-24)


### Bug Fixes

* support multiple domains for OIDC auth, fix login page links ([421c3f3](https://github.com/malarscouterna/scoutrustning/commit/421c3f3daa1b96c7bfa137389f412a4a4c1d8fc2))

## [0.9.2](https://github.com/malarscouterna/scoutrustning/compare/v0.9.1...v0.9.2) (2026-05-24)


### Bug Fixes

* support alternate domains without CSRF errors, fix gen-env bare rerun ([1fdc53c](https://github.com/malarscouterna/scoutrustning/commit/1fdc53c8df6b5cc8066bbc70eb975fb3b75edbd4))


### Miscellaneous

* overhaul gen-env.sh value preservation and mode-switch guard ([27d8a1d](https://github.com/malarscouterna/scoutrustning/commit/27d8a1d94cb97be9d8292002a71a43c4c35e9fc8))


### Documentation

* **web:** enrich login page with description, links, and open /guide publicly ([92d9429](https://github.com/malarscouterna/scoutrustning/commit/92d94299a799e51a61b161f335b529bedaf6b727))

## [0.9.1](https://github.com/malarscouterna/scoutrustning/compare/v0.9.0...v0.9.1) (2026-05-24)


### Miscellaneous

* add dependabot + update all dependencies ([2145820](https://github.com/malarscouterna/scoutrustning/commit/2145820c0fc409ccd648df26dddd63d928dab927))
* **deps-dev:** bump @sveltejs/kit from 2.55.0 to 2.60.1 in /web ([5b9c435](https://github.com/malarscouterna/scoutrustning/commit/5b9c435541ffd812a01026caf592d9e5e60b9b3f))
* **deps:** batch dependabot + apply safe updates ([7ce602b](https://github.com/malarscouterna/scoutrustning/commit/7ce602b648cfdd12a772e1816069e681f7e7194e))
* expand release-please changelog sections ([7d4774f](https://github.com/malarscouterna/scoutrustning/commit/7d4774f656a6a0b6893bf4cb5b832ee49608dd9e))
* rename project to scoutrustning ([8d1c93c](https://github.com/malarscouterna/scoutrustning/commit/8d1c93c5e0c1b46b5bef40a4a56214dde9d415a8))


### Documentation

* move AI assistant setup into Contributing section ([c245419](https://github.com/malarscouterna/scoutrustning/commit/c24541959e8b1834d389d8c25325b2961b2f643f))
* relicense from AGPL-3.0 to MIT ([bdd0bcd](https://github.com/malarscouterna/scoutrustning/commit/bdd0bcd2de455fcba662fab3876094a2797c3cb7))
* update project name to scoutrustning across README and docs ([5837d76](https://github.com/malarscouterna/scoutrustning/commit/5837d76d59faf96e44bf8a623b6571e42d5e7ce8))

## [0.9.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.8.1...v0.9.0) (2026-05-24)


### Features

* **api:** notifications step 1-3 — migration, group members API, issue assignees ([242db0c](https://github.com/malarscouterna/ms-utrustning/commit/242db0c010e74a7d4fea695abe5575871a9dd279))
* **api:** notifications step 4.5–5 — demo mode protection, MeHandler, notification prefs API ([4643d77](https://github.com/malarscouterna/ms-utrustning/commit/4643d7783cbe8626ae9c84f3248ebfa0a13c7129))
* **api:** notifications step 4.5–5 — demo mode protection, MeHandler, notification prefs API ([e52b8ec](https://github.com/malarscouterna/ms-utrustning/commit/e52b8ec11569dce476aa7c65273b5b8036f63c78))
* notification preferences UI ([c01ede7](https://github.com/malarscouterna/ms-utrustning/commit/c01ede77ec024ead81109b39dec4abe9e0db441c))
* **notifications:** GChat team mapper UI + member space linking ([6555d9f](https://github.com/malarscouterna/ms-utrustning/commit/6555d9fd2f38c45ab66846b0e12f60dad66214b0))
* **notifications:** personal notif email override + UX & terminology polish ([4da5ba2](https://github.com/malarscouterna/ms-utrustning/commit/4da5ba2b4b1d67681c0d2d0374531308aee94bf6))
* **notifications:** phase 3.5a backend — three-tier prefs, threading, broadcast, team settings ([6efe9bb](https://github.com/malarscouterna/ms-utrustning/commit/6efe9bb9d5866ec4c4431fc8215772d236b437e9))
* **notifications:** phase 3.5a UI — team settings, team_default label, force defaults ([7d767d3](https://github.com/malarscouterna/ms-utrustning/commit/7d767d3a37f1e38253656a1ca05af0bd27a24608))
* **notifications:** phase 3.5b — GChat bot, team settings UI, group defaults, notification UX fixes ([3b39ecc](https://github.com/malarscouterna/ms-utrustning/commit/3b39ecc8e2509a7853bd2cf794f8e068321450bb))
* **notifications:** phase 3.6 — Gruppkanal, personal email policy, nullable team channel selection ([36d4134](https://github.com/malarscouterna/ms-utrustning/commit/36d4134fc87ade8a637528cf40db2b0ce1aad722))
* **notifications:** Phase 3.7 — issue broadcast parity + GChat two-message threading ([15c830d](https://github.com/malarscouterna/ms-utrustning/commit/15c830dc1fd8f6be062d40530d3c7974a48fe6e2))
* **notifications:** SMTP settings, group defaults by role, dev email infra ([54baa60](https://github.com/malarscouterna/ms-utrustning/commit/54baa604d85ca2819b679f74006aca81046ec8f6))
* **notifications:** step 10 - email body templates (in progress) ([efacd1d](https://github.com/malarscouterna/ms-utrustning/commit/efacd1d44f56b287ccab5e33f23f1981dc3dadbc))
* **notifications:** step 10 - email template polish and bug fixes ([d40c970](https://github.com/malarscouterna/ms-utrustning/commit/d40c97019f785aac9d239ae37b6723f7662792db))
* **notifications:** step 6.5 + 7 — team_ids, SMTP notifier, event-triggered sends ([4c595d7](https://github.com/malarscouterna/ms-utrustning/commit/4c595d759cd62f4fa21aba7c5e13837d4485c166))
* **notifications:** steps 7.5 + 9 — test email, SMTP UI, group defaults UI ([68cee00](https://github.com/malarscouterna/ms-utrustning/commit/68cee00a7d3f646612d14574dbb927deb0c8027b))
* **notifications:** steps 8 + 11 — scheduler and group logo ([c13dfae](https://github.com/malarscouterna/ms-utrustning/commit/c13dfaeb5c7f0bf1fc32ef591cbf6f12cdce8487))
* **notifications:** team email edit for members + demo mode guards ([6602ce9](https://github.com/malarscouterna/ms-utrustning/commit/6602ce90c8b02baa640e191db173b3fce3ee4a0d))
* **web:** issue assignee picker UI (step 4) ([82272b9](https://github.com/malarscouterna/ms-utrustning/commit/82272b97e7dd66631ebae928dedfe87e09abf48b))


### Bug Fixes

* add structured API errors with i18n translations on the frontend ([e5ca6b8](https://github.com/malarscouterna/ms-utrustning/commit/e5ca6b85247b2898212105cc6e7738eee9b53853))
* **frontend:** resolve svelte-check errors in profile/+page.svelte ([d938471](https://github.com/malarscouterna/ms-utrustning/commit/d938471cf3dcc1cb563d5b7201a89daec4cb8824))
* **notifications:** align prefs API with phase 3.6 shape; document GChat gaps ([ceac8b3](https://github.com/malarscouterna/ms-utrustning/commit/ceac8b3c032feb7b052e77efcb7fbbb3dc6d944f))
* **notifications:** harden demo mode + add dev GChat seed support ([0843c22](https://github.com/malarscouterna/ms-utrustning/commit/0843c226ab4df08dbd61ec6dfbd8eb7330206edf))
* **notifications:** issue dispatch, GChat threading + link formatting ([8bd8f11](https://github.com/malarscouterna/ms-utrustning/commit/8bd8f115c06e840e9d32d4f7f7780b164ec822bb))
* **notifications:** render test email with branded template ([c9d4fdd](https://github.com/malarscouterna/ms-utrustning/commit/c9d4fdd8758b2c2527ae7f3c7e5229f8b7f44be7))
* **notifications:** simplify team notif UI, fix gruppkanal empty-array bug ([30c74a2](https://github.com/malarscouterna/ms-utrustning/commit/30c74a20d916c0bf991d36fc1814eef72820b7bd))

## [0.8.1](https://github.com/malarscouterna/ms-utrustning/compare/v0.8.0...v0.8.1) (2026-04-22)


### Bug Fixes

* **web:** fix Docker production build - copy i18n messages into build context ([7a8fa39](https://github.com/malarscouterna/ms-utrustning/commit/7a8fa390be4783738cb581434584ab19e8dea983))

## [0.8.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.7.0...v0.8.0) (2026-04-22)


### Features

* i18n foundation and backend string localisation ([ecc2b51](https://github.com/malarscouterna/ms-utrustning/commit/ecc2b51810d033251c1e4c55045b42ff679f58eb))
* **i18n:** Phase 4 partial + Phase 5 + language switching fixes ([606b2c8](https://github.com/malarscouterna/ms-utrustning/commit/606b2c84c02cd740187a32d1b7dca648474da574))
* **i18n:** Phase 4 shared components + air json watch ([aa606b7](https://github.com/malarscouterna/ms-utrustning/commit/aa606b7315f447b41e99e49bed862adfcb6025e0))
* **web:** complete i18n phase 4 - migrate all route and component strings to Paraglide ([6e77a6a](https://github.com/malarscouterna/ms-utrustning/commit/6e77a6ab92c9f0255d7ad849bf5c756b35ecb626))
* **web:** Phase 3 i18n - replace labels.ts with Paraglide ([64477a7](https://github.com/malarscouterna/ms-utrustning/commit/64477a7b404697aed3b426b5eb7f601f86b3d95e))

## [0.7.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.6...v0.7.0) (2026-04-21)


### ⚠ BREAKING CHANGES

* **web:** pickup/return flow revamp - UX fixes and state bugs
* **web:** issues revamp - first-class issue entities (frontend)
* **api:** see above

### Features

* **api:** promote issues to first-class entities (backend) ([a55a78b](https://github.com/malarscouterna/ms-utrustning/commit/a55a78b02c2af6d5e63ae5fa763df69346b113e0))
* **web:** issues revamp - first-class issue entities (frontend) ([64edbee](https://github.com/malarscouterna/ms-utrustning/commit/64edbee7553fdae99897cdbde08613da8e26d6cb))
* **web:** pickup/return flow revamp - UX fixes and state bugs ([579a76c](https://github.com/malarscouterna/ms-utrustning/commit/579a76cb122478b3720cef39d9d7072b87d92236))


### Bug Fixes

* test, type-check, and docs sign-off for pickup/return revamp ([9909dc4](https://github.com/malarscouterna/ms-utrustning/commit/9909dc4ad0f3f5931cd3aa727f79eccb34f36bd9))
* **web:** align pickup/return issue reporting to ReportIssueSheet pattern ([c27f308](https://github.com/malarscouterna/ms-utrustning/commit/c27f3084292f77c41b0e389053b2ca3ed90c72da))
* **web:** improve pickup checklist swap UX and status awareness ([9049cc4](https://github.com/malarscouterna/ms-utrustning/commit/9049cc4c934bc600794e9fa216a8721f4693586e))

## [0.6.6](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.5...v0.6.6) (2026-04-17)


### Bug Fixes

* **web:** clear stale Auth.js cookies on login redirect to break redirect loop ([9c06dc4](https://github.com/malarscouterna/ms-utrustning/commit/9c06dc4017635872deed0f651b4f22e0319f0150))

## [0.6.5](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.4...v0.6.5) (2026-04-17)


### Bug Fixes

* **web:** add Secure attribute when deleting __Secure- prefixed Auth.js cookies ([3b36547](https://github.com/malarscouterna/ms-utrustning/commit/3b365472baa183d3aec2d71210e5c247524e19c3))

## [0.6.4](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.3...v0.6.4) (2026-04-17)


### Bug Fixes

* **web:** clear stale Auth.js cookies before login redirect to break redirect loop ([f08d6cd](https://github.com/malarscouterna/ms-utrustning/commit/f08d6cd45df286b171a13cf3045f129afdb81134))
* **web:** fix dashboard card overflow on narrow screens, remove footer ([fc31e31](https://github.com/malarscouterna/ms-utrustning/commit/fc31e319e90236b0c29cdc1eb34b06ad5ea0667e))

## [0.6.3](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.2...v0.6.3) (2026-04-15)


### Bug Fixes

* **web:** browse UX polish, sticky layout, camping cart icon ([65f0866](https://github.com/malarscouterna/ms-utrustning/commit/65f08667ee951a24cdb1ac3797e70e8a9f713a39))

## [0.6.2](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.1...v0.6.2) (2026-04-15)


### Bug Fixes

* **web:** context-aware nav breadcrumb, approval badges, and availability fix ([fdd236f](https://github.com/malarscouterna/ms-utrustning/commit/fdd236fd96ff917111730a6c8687d6f85b6864b2))

## [0.6.1](https://github.com/malarscouterna/ms-utrustning/compare/v0.6.0...v0.6.1) (2026-04-15)


### Bug Fixes

* **web:** load scout web components from static instead of bundled import ([a49ce3c](https://github.com/malarscouterna/ms-utrustning/commit/a49ce3cca786a99f87a51637c7f7cdd954d2cb86))

## [0.6.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.5.1...v0.6.0) (2026-04-15)


### Features

* **web:** ux revamp - dashboard, floating cart, and real-time item management ([72178bc](https://github.com/malarscouterna/ms-utrustning/commit/72178bc791899ec9b68c01820348968b77962ad5))


### Bug Fixes

* **web:** decode jwt token payload as utf-8 instead of latin-1 ([cce71c0](https://github.com/malarscouterna/ms-utrustning/commit/cce71c05379347b9ee3670d891e0f02f09df54ff))

## [0.5.1](https://github.com/malarscouterna/ms-utrustning/compare/v0.5.0...v0.5.1) (2026-04-13)


### Bug Fixes

* add images package to git (was excluded by overly broad .gitignore) ([6884ce0](https://github.com/malarscouterna/ms-utrustning/commit/6884ce001c0f80a1851fc90ec6e5afd78e65a03c))

## [0.5.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.4.1...v0.5.0) (2026-04-13)


### ⚠ BREAKING CHANGES

* units table renamed to teams, role-mapping.json replaced by DB-driven team_claim_mappings, API route /units → /teams, used_by_unit_id → used_by_team_id.

### Features

* **api:** image upload, processing, and serving infrastructure ([b98ddad](https://github.com/malarscouterna/ms-utrustning/commit/b98ddad3c634466201c066d6cec4138a26beadf4))
* image attachment on article events ([e0391ea](https://github.com/malarscouterna/ms-utrustning/commit/e0391ea1761adafb0951a646a7e2b5103bffa264))
* manager settings UI, configurable permissions, auto-create teams from OIDC ([34cfe15](https://github.com/malarscouterna/ms-utrustning/commit/34cfe159013ef8ec825e63e4ee195105d04b06d1))
* multi-image support with upload UI and PhotoSwipe gallery ([c5bb17e](https://github.com/malarscouterna/ms-utrustning/commit/c5bb17e64fe20f768f416d115415aa9783034377))
* product_images table, crop UI, attribution, upload permissions ([a78d610](https://github.com/malarscouterna/ms-utrustning/commit/a78d61090cc398c4b75a408fdfb04bcdd43084ba))
* replace role-based access with per-team access levels ([7de78ef](https://github.com/malarscouterna/ms-utrustning/commit/7de78efb562bf259259d38a016fa0f67cb138a62))
* **web:** display product images in browse and article detail ([5ce9162](https://github.com/malarscouterna/ms-utrustning/commit/5ce9162f08136b363c9a7295075654bb271ab268))
* **web:** shared image browser, metadata editing, and display improvements ([022c692](https://github.com/malarscouterna/ms-utrustning/commit/022c692c3127c076afe6837e9a74b83895d2301f))
* **web:** show images and descriptions across all booking views ([1b2d84b](https://github.com/malarscouterna/ms-utrustning/commit/1b2d84ba51e95316b40cee00c3f52ef52b916818))


### Bug Fixes

* **images:** deduplicate shared images, fix article links, improve UX ([54d9b7a](https://github.com/malarscouterna/ms-utrustning/commit/54d9b7a1e9b151fbced038444c57e2b4ec95e29e))


### Performance Improvements

* **api:** increase image quality and resolution ([a507ced](https://github.com/malarscouterna/ms-utrustning/commit/a507ced41b63cf4d19b99b037fede90af45a7ead))

## [0.4.1](https://github.com/malarscouterna/ms-utrustning/compare/v0.4.0...v0.4.1) (2026-04-10)


### Bug Fixes

* **web:** add @types/node to fix 22 svelte-check type errors ([18691a6](https://github.com/malarscouterna/ms-utrustning/commit/18691a64af9200b7b1fde9b6a00decedc8ed40f1))
* **web:** fix SSR crash on booking detail page ([b59eb70](https://github.com/malarscouterna/ms-utrustning/commit/b59eb701378fbdedc3c0d5d7b58e1be79d526f6e))

## [0.4.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.3.1...v0.4.0) (2026-04-10)


### Features

* **web,api:** browse info toggle, inline count, article comments, per-item list ([77ee4d4](https://github.com/malarscouterna/ms-utrustning/commit/77ee4d405421d4f1422632946131162e536e80ec))
* **web,api:** bulk actions toolbar with one-action-at-a-time UX ([5b3bb1a](https://github.com/malarscouterna/ms-utrustning/commit/5b3bb1aa2fa37f24bb5dea3662aa42bacc20953c))


### Bug Fixes

* **web:** quantity tracked count change from edit page, cleanup duplicated labels ([48d2c52](https://github.com/malarscouterna/ms-utrustning/commit/48d2c526cd7325eec558386ffa6b59bc0a449f0c))

## [0.3.1](https://github.com/malarscouterna/ms-utrustning/compare/v0.3.0...v0.3.1) (2026-04-09)


### Bug Fixes

* add missing files from prior commit ([bd019a9](https://github.com/malarscouterna/ms-utrustning/commit/bd019a99336f599ba46fc908e2ccae01fcafe88c))

## [0.3.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.2.2...v0.3.0) (2026-04-09)


### Features

* inventory management foundation — settings, article forms, detail page ([794d252](https://github.com/malarscouterna/ms-utrustning/commit/794d25211cf23e975d44710d83a140bc51b94a4f))
* **web,api:** browse page article links, edit forms with shared/per-item fields, bulk API ([f16783c](https://github.com/malarscouterna/ms-utrustning/commit/f16783cd6e0f4852bb7ae12f1bc3cb647074af25))


### Bug Fixes

* **web:** resolve pre-existing Svelte 5 reactivity warnings and type errors ([cd1401b](https://github.com/malarscouterna/ms-utrustning/commit/cd1401bdebe035a89761d36b24656599174541f3))

## [0.2.2](https://github.com/malarscouterna/ms-utrustning/compare/v0.2.1...v0.2.2) (2026-04-07)


### Bug Fixes

* return 403 instead of 500 for users from unconfigured groups ([adea258](https://github.com/malarscouterna/ms-utrustning/commit/adea258da55c0517d769895a3d71534608d85bea))
* **web:** mobile responsiveness overhaul ([d47a236](https://github.com/malarscouterna/ms-utrustning/commit/d47a23615b2ec17f28d208dbc9d4cda875bf28f9))

## [0.2.1](https://github.com/malarscouterna/ms-utrustning/compare/v0.2.0...v0.2.1) (2026-04-06)


### Bug Fixes

* improve mobile layout with bottom navigation bar ([29d9baa](https://github.com/malarscouterna/ms-utrustning/commit/29d9baa2e9b65a704efe694cba2ebe533f4c75fa))

## [0.2.0](https://github.com/malarscouterna/ms-utrustning/compare/v0.1.0...v0.2.0) (2026-04-06)


### Features

* add Swedish usage guide at /guide ([b424fab](https://github.com/malarscouterna/ms-utrustning/commit/b424fabd53501d5b674e716ab1a52df540f2fb53))
* browse page availability display and booking flow improvements ([61d3552](https://github.com/malarscouterna/ms-utrustning/commit/61d3552823a1d5937b8736526b0c42e14d5c8aa4))
* CSV import count column, browse page tracking mode differentiation ([bd10177](https://github.com/malarscouterna/ms-utrustning/commit/bd10177a3ef89b297d1d707f5f8b5b230d88150d))
* demo mode, env generator, and deployment hardening ([69fd31d](https://github.com/malarscouterna/ms-utrustning/commit/69fd31d4161774ed254d34b3b031e37a8f8ba183))
* **dev:** add hot reload for Go API and SvelteKit in Docker ([91d9d55](https://github.com/malarscouterna/ms-utrustning/commit/91d9d55b0fed74169d33ac0248e527f4eea9bd62))
* event history limit, draft cleanup, pickup revert ([2ad7853](https://github.com/malarscouterna/ms-utrustning/commit/2ad7853d90984e6f11426537177b384ae74662ad))
* refactor article status to separate condition from booking state ([fc4943b](https://github.com/malarscouterna/ms-utrustning/commit/fc4943b8504c8e465637a5118443f1aa3b7d0ee1))
* three-level approval flow with booking event history ([e1badaa](https://github.com/malarscouterna/ms-utrustning/commit/e1badaa808c43d497e72ecb797042622716da3eb))


### Bug Fixes

* dev persona cleanup, issues page scoping, seed script ([5d01202](https://github.com/malarscouterna/ms-utrustning/commit/5d01202d5965b8e62a9e5a3f2622903dc26df4ed))


### Performance Improvements

* **api:** share single Postgres container across all integration tests ([9d95d2d](https://github.com/malarscouterna/ms-utrustning/commit/9d95d2d31f230dba8c2882c1b57282676eea4801))

## 0.1.0 (2026-04-04)


### Features

* access control, dev persona switcher, and unit/project model ([ffa5624](https://github.com/malarscouterna/ms-utrustning/commit/ffa5624dcff6370a98e6c46da6b96f6476c3ecf8))
* API foundation — auth middleware, sqlc, test harness ([dd1b4bc](https://github.com/malarscouterna/ms-utrustning/commit/dd1b4bcfd2336b60ce90622114d6a361b0670a06))
* article browsing, API filtering, v0 versioning ([5b8a403](https://github.com/malarscouterna/ms-utrustning/commit/5b8a403cf56c2705aacb28f53bb6edee42ecccbe))
* article CRUD, CSV import, location/category management ([36ea3d7](https://github.com/malarscouterna/ms-utrustning/commit/36ea3d798e82d3b0a9c165d71ee349a541b22fbe))
* booking create, submit, update with availability and access control ([72692a8](https://github.com/malarscouterna/ms-utrustning/commit/72692a80c8d203f3192937f12dfd7894a0befaa5))
* booking pickup checklist with per-item status, swap, and quantity-tracked support ([146f041](https://github.com/malarscouterna/ms-utrustning/commit/146f041c60ff41ba1681a992ebbb3b9a5c432fb2))
* booking UI, copy, cancel, location-scoped availability ([0592f88](https://github.com/malarscouterna/ms-utrustning/commit/0592f886751bd0fb1e8d010a8aaac24ed5edca2c))
* initial project scaffold ([6c4828b](https://github.com/malarscouterna/ms-utrustning/commit/6c4828ba8e3b78981aacbfaa5abcb0bf658828e4))
* issue reporting via article events and unified status endpoint ([f1d734c](https://github.com/malarscouterna/ms-utrustning/commit/f1d734c3727daa5f674be3e4b287a9eb4406464c))
* OIDC authentication with ScoutID (Phase 3 Step 1) ([e69108d](https://github.com/malarscouterna/ms-utrustning/commit/e69108d666c30c78b4a8948be81527027c189918))
* pickup checklist and return flow with quantity-tracked support ([4cb8223](https://github.com/malarscouterna/ms-utrustning/commit/4cb8223b7b679484823297aab5011f96b42d4e45))
* Step 4 polish — approval badges, shared components, booking UX ([70b09a3](https://github.com/malarscouterna/ms-utrustning/commit/70b09a3599e32488f671085e6fffb9f4dc181475))
