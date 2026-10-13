# To Source: canon 0.1.7 runtime header (2026-10-13)

From the Canon compiler session. Once Scripts moves the canon pin to 0.1.7 and Resource runs
`canon build`, Resource rewrites 12 generated files in Source:
`Engine/Resource/Generated/{character,combat,content,economy,engine,events,guild,items,pet,progression,social}/canon_runtime.h`
and `Engine/Telemetry/Generated/canon_runtime.h`.

**Do:** commit them with Resource's build, and run the Windows build.
- This is the header-only C++ runtime. 0.1.6 adds decoders that refuse repeated keys and name
  the first failure at a `$`-rooted path, plus open enums and `@text` decoders.
- No generated API that Source code calls was renamed or removed, and no Source code matches the
  changed messages.
- **Not verified on your toolchain:** canon's tests compile this header with clang and libc++ on
  Linux only. Your Windows build is the real check; report any error to canon, with the
  compiler's message.

Nothing else for Source. Messages that existing generated decoders may now print (`duplicate
key`, `invalid JSON at byte N`) appear only on malformed JSON.
