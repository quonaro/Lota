# Changelog
All notable changes to this project will be documented in this file. See [conventional commits](https://www.conventionalcommits.org/) for commit guidelines.

- - -
## [Unreleased](https://github.com/quonaro/lota/compare/v1.6.1..HEAD)
#### Bug Fixes
- fix install.sh to match build.sh artifact naming (amd64/arm64 instead of x86_64/aarch64, remove .tar.gz extension) - (quonaro)

- - -
## [v1.6.1](https://github.com/quonaro/lota/compare/v1.6.0..v1.6.1) - 2026-04-21

- - -
## [v1.5.0](https://github.com/quonaro/lota/compare/fd263bf15328a8f47767ade734f6b756da0b6ab0..v1.5.0) - 2026-04-21
#### Features
- add installation script with checksums verification and interactive setup - ([f554fde](https://github.com/quonaro/lota/commit/f554fde028f8420501a6c09b89a65223ac41b142)) - quonaro
#### Miscellaneous Chores
- update changelog for v1.4.0 - ([fd263bf](https://github.com/quonaro/lota/commit/fd263bf15328a8f47767ade734f6b756da0b6ab0)) - quonaro

- - -

## [v1.4.0](https://github.com/quonaro/lota/compare/436e1518859eccd8b765e82e308cb0a699865443..v1.4.0) - 2026-04-20
#### Features
- add YAML file import support with prefix and section selection - ([43af8d7](https://github.com/quonaro/lota/commit/43af8d787434480cfd3830d429ebb035dd1f4e2c)) - quonaro
#### Documentation
- add hyperlink to LICENSE file in README - ([dc462cb](https://github.com/quonaro/lota/commit/dc462cb8af7f6eee5243ef0cd8c006de9bce0e45)) - quonaro
- expand README with features, examples, and comprehensive usage guide - ([21a4f7b](https://github.com/quonaro/lota/commit/21a4f7b6870dfe7aa8a241b6ca912add8c241ae6)) - quonaro
- redefine AI agent role from code writer to mentor in workspace rules - ([6c9ecb4](https://github.com/quonaro/lota/commit/6c9ecb48a9d99e9d2ca77d900bf0c805586a029a)) - quonaro
#### Refactoring
- improve flag handling in command resolution and remove filterGlobalFlags - ([bfe5e6d](https://github.com/quonaro/lota/commit/bfe5e6d36ed02ee6890289d1aeef1c5d61ecf229)) - quonaro
#### Miscellaneous Chores
- remove Windsurf AI workspace rules and ignore AI agent directories - ([316b3c8](https://github.com/quonaro/lota/commit/316b3c827aee0e868991a6a50b824e197fb77a3e)) - quonaro
- add Windsurf AI development protocol and improve help system with verbose mode - ([e6a7836](https://github.com/quonaro/lota/commit/e6a78364b054ffe5d6fd7f8412c58d47fa5c012a)) - quonaro
- update changelog for v1.3.0 - ([436e151](https://github.com/quonaro/lota/commit/436e1518859eccd8b765e82e308cb0a699865443)) - quonaro

- - -

## [v1.3.0](https://github.com/quonaro/lota/compare/5de88d377cbc69336252aa9112d747b6ac3d0e77..v1.3.0) - 2026-04-13
#### Features
- rename darwin to macos in build output filenames - ([c49afe2](https://github.com/quonaro/lota/commit/c49afe20fa6b32c9a0a535eff0d0d181772f5515)) - quonaro
- add configuration validation with warnings for missing env files - ([3b2de93](https://github.com/quonaro/lota/commit/3b2de9399894fe634035152db668e5503fc89056)) - quonaro
- add environment file import support with @import directive - ([578b715](https://github.com/quonaro/lota/commit/578b7153fef17e3d3b585aa6f92ba5ec032ca67a)) - quonaro
- add hierarchical shell configuration with OS-specific defaults - ([5de88d3](https://github.com/quonaro/lota/commit/5de88d377cbc69336252aa9112d747b6ac3d0e77)) - quonaro
#### Refactoring
- change ConfigValidator to use pointer receiver and fix shell command parsing - ([5406dc3](https://github.com/quonaro/lota/commit/5406dc3853bed855081a6521baca7d88385f8e6f)) - quonaro

- - -

## [v1.2.0](https://github.com/quonaro/lota/compare/0da49f2123aa3012b305a4ec39df75df0ea23836..v1.2.0) - 2026-04-10
#### Features
- add context-aware help for specific commands - ([3bd7d49](https://github.com/quonaro/lota/commit/3bd7d49ec49789eae1bc7c28328c14cb854a69c5)) - quonaro
#### Refactoring
- replace ParseCommandPath and FindCommand with greedy ResolveCommand - ([ac86bb2](https://github.com/quonaro/lota/commit/ac86bb2770d6befa64022b76d3c5bde43ad3b4fe)) - quonaro
#### Miscellaneous Chores
- update changelog for v1.1.0 - ([0da49f2](https://github.com/quonaro/lota/commit/0da49f2123aa3012b305a4ec39df75df0ea23836)) - quonaro

- - -

## [v1.1.0](https://github.com/quonaro/lota/compare/0826a66b38d5a796bea1e6081f012e198128ba60..v1.1.0) - 2026-04-10
#### Features
- add ASCII art banner and colored output to version command - ([0826a66](https://github.com/quonaro/lota/commit/0826a66b38d5a796bea1e6081f012e198128ba60)) - quonaro
#### Bug Fixes
- correct syntax in post-commit hook - ([c3207fb](https://github.com/quonaro/lota/commit/c3207fb20476ac7c07afe1f0d97814e36753ec0d)) - quonaro
#### Refactoring
- improve flag validation and code cleanup - ([0298036](https://github.com/quonaro/lota/commit/029803619ef5847ad7735933af381d1c73325a63)) - quonaro
#### Miscellaneous Chores
- add post-commit hook for automatic tagging - ([6608a02](https://github.com/quonaro/lota/commit/6608a0257d75a7b4e0b6171730ea5173ee8f2b1c)) - quonaro

- - -

## [v1.0.0](https://github.com/quonaro/lota/compare/2d9727bce8fc0fcf276db571351ca69fa54dc3e1..v1.0.0) - 2026-04-10
#### Features
- initial commit - ([2d9727b](https://github.com/quonaro/lota/commit/2d9727bce8fc0fcf276db571351ca69fa54dc3e1)) - quonaro
#### Tests
- add comprehensive unit tests for CLI parsing and command resolution - ([7941b5a](https://github.com/quonaro/lota/commit/7941b5a82862ad243072f1f8ef5ad6e84be68df4)) - quonaro
#### Refactoring
- improve error handling and output consistency - ([2094898](https://github.com/quonaro/lota/commit/209489813cef1ada618739fb454a55afaae07579)) - quonaro
#### Miscellaneous Chores
- move build.sh to scripts and add bump and install-hooks scripts - ([ac43b44](https://github.com/quonaro/lota/commit/ac43b446b4820a1c1cefed82156891791ce40527)) - quonaro
- add cocogitto config for automated versioning - ([4c84e47](https://github.com/quonaro/lota/commit/4c84e47b66b073f12681327740ba9691244582dc)) - quonaro

- - -

Changelog generated by [cocogitto](https://github.com/cocogitto/cocogitto).