# Changelog

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
