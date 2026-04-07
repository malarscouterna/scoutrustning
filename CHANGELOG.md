# Changelog

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
