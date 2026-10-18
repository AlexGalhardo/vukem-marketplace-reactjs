# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.8.0] - 2026-10-18

### Changed

- **repo:** Rename the repository to vukem-marketplace-reactjs ([0ad9d81](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/0ad9d81046f276f20acd062b3c92052e944800f8))
- **deps:** Bump vukem to v0.28.0 and its graphql, stripe, otel and mongo modules ([533dc36](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/533dc360fab839c9dd4e48ce141dc30499784619))

## [0.7.0] - 2026-10-18

### Added

- **chat:** Add buyer and seller chat over broadcast websockets ([15590db](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/15590dbfe2696293e0d33cde2551e59656355ee3))
- **chat:** Seed demo conversations between demo buyers and sellers ([aa43c72](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/aa43c72eb391d93ce60f9c02dd6507977357cb95))
- **chat:** Add inbox, conversation and admin pages with live updates ([cfaf3a3](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/cfaf3a3d260482d8e64975ee46c0755bbc0af39b))

### Changed

- **deps:** Bump vukem to v0.27.0 and adopt its error pages, problem+json and lazy stores ([6e8803c](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/6e8803c99bd28b7e876be4d02925aafb8d6812f4))
- **admin:** Move feature flags onto vukem flags ([41ffb0d](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/41ffb0d7994431d046c26599d9fc503e081c5b76))

## [0.6.0] - 2026-10-18

### Added

- **reviews:** Add review moderation columns, helpful votes and a contact filter ([5e56b0a](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/5e56b0aa38a6f60ef74e404ac45545be65fb9ba5))
- **reviews:** Let buyers review paid order lines, vote, reply and moderate ([c7393a6](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/c7393a6fb10ed7655a7ba735e2b0dd26dc309868))
- **reviews:** Show reviews on the product page with form, votes, replies and moderation ([8d4b7e5](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/8d4b7e5ca872527a9e61942c5b6927d11536085c))
- **catalog:** Add product questions and answers ([76848f6](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/76848f63651b8301388da2f391105378886da37c))
- **ui:** Add product questions section, seller inbox and admin moderation pages ([acd605b](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/acd605becc2fb54d9f277506d62625e964d93733))
- **catalog:** Simulate demo purchases, reviews and questions in demo:catalog ([31fd2fa](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/31fd2faff62f99fa72ce359eec323629e6f8e023))

### Fixed

- **reviews:** Select pending review lines without an ambiguous id on postgres ([5129cc6](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/5129cc69d642dcda23de8f0e6d458ff7536bd63c))
- **shops:** Send users without a shop to /sell and render inertia error pages ([a5bb200](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/a5bb2006bec53a3c4f76f02763f30262a873b2a5))

## [0.5.0] - 2026-10-17

### Added

- **catalog:** Seed a deterministic demo catalog of 100 sellers and 2000 products ([05a8662](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/05a8662f5ff3c7d22933576f06e51625d5649149))
- **infra:** Seed the development stack and document the demo accounts ([32fabcd](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/32fabcde8185ef8848a29bf9aaf2cb1beb559132))
- **admin:** Add admin-managed feature flags with an audit log ([d38eb12](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/d38eb12190ab5064570d2eeb8b499ee3b40a3ad7))
- **auth:** Add continue with google and harden google account linking ([a36f343](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/a36f3435d3abe67d26eb0a3859bbc2408e0d1934))
- **auth:** Normalize and validate names on sign-up and profile ([3e2738b](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/3e2738b2367c697ec6bde001856e819efdf437b2))
- **catalog:** Carry the rating on product cards ([efc9122](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/efc9122c8fe21cf61cc74307d545afc65d1bb327))
- **ui:** Marketplace layout in the big LatAm marketplace style, own brand ([2b7900c](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/2b7900cb386cbf4767f9651233c0b671820e9b23))

### Changed

- **deps:** Bump vukem to v0.26.0 and use typed stripe accounts v2 ([aebf30d](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/aebf30d72612963a524f25ef2a2644025b893705))

## [0.4.0] - 2026-10-17

### Added

- **infra:** Export otel metrics and drop the custom db and queue spans ([52bec68](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/52bec6887f6f6e1eb7b0151f8934192cb4833076))

### Changed

- **deps:** Bump vukem to v0.25.0 and adopt queue in serve, rebind, uuid and app seeders ([88aabb1](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/88aabb1f8add2c53544c529afcfdd8928b807f05))
- **infra:** Open redis and mongo lazily and serve /ready from app.ReadyHandler ([139d499](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/139d4996b5f66bbd92553a60b004371fe5a47e57))
- **catalog:** Register catalog routes after boot and pass the mongo client directly ([0a504d3](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/0a504d358245feeb521a05b53bffe7611f11266d))
- **auth:** Rely on vukem keyed errors instead of matching english messages ([045b1c4](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/045b1c4d8b1852de09219ce8c47668f07d8a46bc))
- **auth:** End sessions with vukem session versions instead of the password stamp ([077e3e2](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/077e3e2b4c143c3da65e021d37211ee22d1957d3))
- **payments:** Send stripe v1 calls through contrib/stripe v0.2.0 ([d255f7a](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/d255f7a20f18ffb4a5ffc040dea0673b2ce56139))
- **api:** Drop graphql camelFields now that contrib/graphql folds go names ([58e9b90](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/58e9b90c1a269339c9b22a67b7bcb639eb4da32b))
- **api:** Serve the v1 openapi document with ServeOpenAPIWith ([9c112e4](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/9c112e4ce7e5310dab78b8a7d267e36d1ea7e7c4))
- **api:** Serve grpc on its own router with contrib/grpc Server ([9d2c7fe](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/9d2c7fed8eaa465e7619e9cd945ffa10935b4016))
- **infra:** Send telegram alerts through vukem SendSync and turn log forwarding off ([5e81573](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/5e81573d8520c17860c384785033e180160fa083))
- **catalog:** Create the local bucket with drivers/s3 Disk.CreateBucket ([db2b740](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/db2b740633bb8430f27102f04860b900cfc3e89c))

### Fixed

- **api:** Answer auth failures as problem+json whatever the accept header ([3b0ef0f](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/3b0ef0f726aab1d89a0f4c6639e88ca9428a33db))

## [0.3.0] - 2026-10-17

### Added

- **catalog:** Add catalog service for shops, products, images and search ([b338355](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/b33835515b904a61d3e8fa7ab77830b4e7dca33c))
- **catalog:** Add storefront and seller pages in React ([bfec939](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/bfec939c3f7b14f96c065f810dba37b128f400c8))
- **cart:** Add server-priced cart, freight table, ViaCEP client and address book ([7195ff4](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/7195ff4548e29cd0b402dbeb5f9c1f9049585ecf))
- **payments:** Add ledger, Stripe Connect and AbacatePay PIX flows and payouts ([14ebdce](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/14ebdce0a6e32df4fd5d0d25bde8519504e9333b))
- **checkout:** Add checkout service, commerce routes and PIX expiry schedule ([82c5945](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/82c594540520328385275b76733ca2e070aecd8a))
- **checkout:** Add cart, address book, checkout, PIX and payout pages ([c5cf49c](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/c5cf49c6ad49ab6f5217bdb4a489ffb6e9c005dd))
- **checkout:** Seed the freight table and document payment env vars ([1c73213](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/1c73213cf0ee978ceaea77325fe211211ede91af))
- **infra:** Schedule token and log pruning, add worker and scheduler services ([07cf80d](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/07cf80d2a642d193fb42fa0e58b5e78171eec2c2))
- **infra:** Trace http, sql and queue jobs with opentelemetry ([25c6112](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/25c6112b5cb185f3a62f88f383956f92fcb3dceb))
- **infra:** Run queue worker and scheduler inside serve by default ([b669e90](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/b669e900d2d78c78af8b56bed8c6f908bbd97b54))
- **infra:** Add contact form and rate-limited 5xx alerts over telegram ([3ec082d](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/3ec082d59e3d9fd818cf1244ee23976d2a339404))
- **api:** Deliver signed seller webhooks with a delivery log ([b4ae9d5](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/b4ae9d5f2ce4c4b8e900121a8bc26c66600155cc))
- **orders:** Stream per-user realtime events over sse and redis pub/sub ([73eb9b3](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/73eb9b3de8df413c7586f921a2f4ccc2e5906e0f))
- **infra:** Wire ops providers, routes and translations ([88df90d](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/88df90ddec377ea21f2b646cca1441425e96c608))
- **catalog:** Mount add-to-cart and freight quote, sync reference data on start ([cbec9da](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/cbec9da892e9ec939db02160af6e36b526ca8aa7))

### Changed

- **catalog:** Make image variants with contrib/imaging in a queue job ([1c94050](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/1c9405041c06cd3cb3689e3c98ed4acc8bb1dea2))
- **deps:** Bump vukem to v0.24.0 with s3, mongo and redis drivers ([f5626dc](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/f5626dcb8adae82a141acba82fd8951045a731ce))

### Fixed

- **ui:** Close the locale menu after picking a language ([bb550f7](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/bb550f7cc74cb4db8ee59e35c2e5e4c587ca92c1))
- **payments:** Create connected accounts with stripe accounts v2 ([1321272](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/132127246bc607850a9f54003b76cd16e430fbc1))
- **payments:** Request card payments with the stripe transfers capability ([9e1a39d](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/9e1a39d2d097cde284a8c2b317ba5a844958ab69))

## [0.2.0] - 2026-10-17

### Added

- **auth:** Port vukem auth scaffold with 2fa and google sign-in ([2f9d80f](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/2f9d80f952223cc32ef09f30e8061d1c6c95074f))
- **infra:** Add catalog, commerce and api schema migrations ([fd62b5a](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/fd62b5a48db45b1ae025471d9be44693113574b9))
- **api:** Add api keys, rest v1, graphql and grpc over consumer-side services ([8bcc863](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/8bcc8631d0aead9dda0cc898c3e18c85384212a2))
- **auth:** Queue verification mail and add verified middleware ([058400c](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/058400c22788e0a22a1783ce03b629f86f5ae1f3))
- **auth:** Harden password reset and revoke sessions on password change ([529aa01](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/529aa013ce21da143143bd90f8e09a0bb955637f))
- **auth:** Harden google sign-in against replay and pre-hijacking ([ebd38ad](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/ebd38ad4c9e5b1960249a2da7ebfe444ce403c8c))
- **auth:** Seed demo buyer, seller and admin accounts ([80d65e3](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/80d65e3d2f115dd364102c54e98ff3eef6e9825b))

### Changed

- **deps:** Bump vukem to v0.22.0 ([ef5f16f](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/ef5f16f3cf70715496fa42d2e1537a70cd324302))
- **deps:** Add contrib/graphql and contrib/grpc v0.1.0 with catalog and orders protos ([22458ee](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/22458ee52a7a4185a383d8ce9f7788021de86617))
- **deps:** Bump vukem to v0.23.0 and drivers/postgres to v0.4.0 ([cebee76](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/cebee7660aed86ab414691de28529d16338b425d))
- **api:** Use app.RateLimiters for api rate limits and document api env ([e168ebd](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/e168ebddcc3c8f4086c5ffd6be2fcb248f2f6089))

### Fixed

- **platform:** Accept MONGO_DATABASE when MONGO_URL has no database ([e315f82](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/e315f8259a876941ebd0ac4687d19db4e6eef600))
- **api:** Disable grpc instead of failing boot when production has no tls ([e5f0ff6](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/e5f0ff662bebcdddefb5ff35a7ac72faa6ec5b33))

### Security

- **auth:** Add account profile and security endpoints with server-side i18n ([d96e6d6](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/d96e6d6958e998151ad50d232c045172e4017494))
- **ui:** Add account pages and translate every auth page ([ccb8efa](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/ccb8efa11d77aab9b60a661f85c6fdff9a27ba00))

## [0.1.0] - 2026-10-17

### Added

- **infra:** Wire vukem app with postgres, redis and mongo ([1bccb23](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/1bccb23a6c1f1856f3e38111bdb2dbb4249b1a06))
- **ui:** Home page with theme toggle and locale switch ([3ba0936](https://github.com/AlexGalhardo/vukem-marketplace-reactjs/commit/3ba093619dd2e2e63a489fe4b6e0a5beb84a66be))

[0.8.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/AlexGalhardo/vukem-marketplace-reactjs/releases/tag/v0.1.0
