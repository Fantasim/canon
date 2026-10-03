# Progen counterexample triage (85 archives)

Replay: go test ./internal/testkit/progen -run TestCounterexamples -v (child replay per archive).

| archive | class | reason | owner | cause group |
|---|---|---|---|---|
| corrupt_corruption_12884904507.txtar | FIXED | replay clean | - | - |
| corrupt_corruption_12884905109.txtar | FIXED | replay clean | - | - |
| corrupt_corruption_12884907455.txtar | FIXED | replay clean | - | - |
| corrupt_corruption_12884919634.txtar | FIXED | replay clean | - | - |
| corrupt_corruption_12884923932.txtar | FIXED | replay clean | - | - |
| corrupt_corruption_12884926146.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589934908.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589937953.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589938068.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589938128.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589938825.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589939473.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589939518.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589939661.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589941382.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589943145.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589945209.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589946246.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589947312.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589951815.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589955371.txtar | FIXED | replay clean | - | - |
| grammar_roundtrip_8589960503.txtar | FIXED | replay clean | - | - |
| mutation_E1101-GRAMMAR-md--2-6--format-spec-_4294967308.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E1107-GRAMMAR-md--2-6--unterminated-string-_4294967314.txtar | STILL FAILING | expected exactly the operator code; got misplaced E1107, E2102/plain | syntax | A: E1107 unterminated string reported with spurious E2102 (lexer recovery) |
| mutation_E1109-GRAMMAR-md--2-6--invalid-escape-_4294968905.txtar | FIXED | replay clean | - | - |
| mutation_E1109-GRAMMAR-md--2-6--invalid-escape-_4294969148.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E1110-GRAMMAR-md--2-4--leading-zero-_4294968010.txtar | FIXED | replay clean | - | - |
| mutation_E1111-GRAMMAR-md--2-5--unit-order-_4294967772.txtar | FIXED | replay clean | - | - |
| mutation_E1112-GRAMMAR-md--2-6--empty-interpolation-_4294968235.txtar | FIXED | replay clean within left line: W1002*4 | - | - |
| mutation_E1112-GRAMMAR-md--2-6--empty-interpolation-_4294970091.txtar | FIXED | replay clean | - | - |
| mutation_E1112-GRAMMAR-md--2-6--empty-interpolation-_4294971731.txtar | FIXED | replay clean | - | - |
| mutation_E1121-GRAMMAR-md--6-7--argument-twice-_4294967328.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E1126-GRAMMAR-md--4-3--none-as-enum-member-_4294968488.txtar | FIXED | replay clean | - | - |
| mutation_E1129-GRAMMAR-md--6-1--brace-literal-in-a-header-_4294967336.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E1129-GRAMMAR-md--6-1--brace-literal-in-a-header-_4294967563.txtar | FIXED | replay clean | - | - |
| mutation_E1130-GRAMMAR-md--5-10--expect-outside-a-test-_4294967337.txtar | FIXED | replay clean within left line: E1117*2, E8009/missing, W1002*2 | - | - |
| mutation_E1134-GRAMMAR-md--5-10--break-outside-a-loop-_4294967341.txtar | FIXED | replay clean within left line: E1116/token*3, E1117*3, E8004/noEmit*2, E8009/missing, W1002*12 | - | - |
| mutation_E1135-GRAMMAR-md--5-10--return-in-a-test-_4294967342.txtar | FIXED | replay clean within left line: E8009/package | - | - |
| mutation_E2001-TYPES-md--3-1--two-packages-in-one-directory-_4294967357.txtar | FIXED | replay clean | - | - |
| mutation_E2103-TYPES-md--10-2--ref-without-collection-_4294967592.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E2103-TYPES-md--10-2--ref-without-collection-_4294967819.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E2103-TYPES-md--10-2--ref-without-collection-_4294968046.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E2103-TYPES-md--10-2--ref-without-collection-_4294969445.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E2105-TYPES-md--3-6--id-on-a-table-element-_4294967367.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E2105-TYPES-md--3-6--id-on-a-table-element-_4294968956.txtar | FIXED | replay clean within left line: E3004/missing, E8009/missing | - | - |
| mutation_E2106-TYPES-md--3-2--declared-twice-_4294967368.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E3003-TYPES-md--3-5--unknown-member-_4294967602.txtar | FIXED | replay clean within left line: E1116/token, E2102/plain, W1002*5 | - | - |
| mutation_E3004-TYPES-md--12-2--too-many-arguments-_4294969192.txtar | FIXED | replay clean within left line: E1117*3, E2102/plain*2, W1002*5 | - | - |
| mutation_E3013-TYPES-md--9-3--table-of-an-enum-_4294967630.txtar | FIXED | replay clean | - | - |
| mutation_E3013-TYPES-md--9-3--table-of-an-enum-_4294967847.txtar | FIXED | replay clean | - | - |
| mutation_E3015-TYPES-md--15--refinement-bound-reads-a-let-_4294967385.txtar | FIXED | replay clean within left line: W1002*2 | - | - |
| mutation_E3015-TYPES-md--15--refinement-bound-reads-a-let-_4294967848.txtar | FIXED | replay clean | - | - |
| mutation_E3016-TYPES-md--12-3--built-in-as-a-value-_4294968073.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain, E3002, W1002*4 | - | - |
| mutation_E3019-TYPES-md--12-1--bare-return-_4294967618.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, W1002*4 | - | - |
| mutation_E3022-TYPES-md--13-1--record-contains-itself--with-a-default-_4294967393.txtar | FIXED | replay clean within left line: E1116/token, E8009/missing, W1002*4 | - | - |
| mutation_E3022-TYPES-md--13-1--record-contains-itself--with-a-default-_4294967625.txtar | FIXED | replay clean | - | - |
| mutation_E3022-TYPES-md--13-1--record-contains-itself--with-a-default-_4294967856.txtar | FIXED | replay clean within left line: E8009/missing, W1002*5 | - | - |
| mutation_E3022-TYPES-md--13-1--record-contains-itself-_4294967392.txtar | FIXED | replay clean | - | - |
| mutation_E3022-TYPES-md--13-1--record-contains-itself-_4294968766.txtar | FIXED | replay clean | - | - |
| mutation_E3025-TYPES-md--13-3--Float-bound-in-a-Range-_4294967395.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain, E3002, W1002*4 | - | - |
| mutation_E3102-TYPES-md--9-1---codes-code-twice-_4294967397.txtar | FIXED | replay clean | - | - |
| mutation_E3102-TYPES-md--9-1---codes-code-twice-_4294970401.txtar | STILL FAILING | expected exactly the operator code; got extra E7111/member | load | B: E7111 emitted when E3102 @codes duplicate present |
| mutation_E3304-TYPES-md--5-2--identifier-map-key-_4294968089.txtar | FIXED | replay clean within left line: E1116/token, E2102/plain, E3002, W1002*4 | - | - |
| mutation_E3305-TYPES-md--5-2--undecidable-brace-literal-_4294968554.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain, E3002, W1002*4 | - | - |
| mutation_E3306-TYPES-md--12-3--function-type-as-a-field-_4294967410.txtar | FIXED | replay clean within left line: E8009/missing*2 | - | - |
| mutation_E3306-TYPES-md--12-3--function-type-as-a-field-_4294967864.txtar | FIXED | replay clean | - | - |
| mutation_E3306-TYPES-md--12-3--function-type-as-a-field-_4294968091.txtar | FIXED | replay clean | - | - |
| mutation_E3307-TYPES-md--12-7--assign-to-a-field-_4294967411.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain, E3002, W1002*4 | - | - |
| mutation_E3313-TYPES-md--14--input-read-at-build-time-_4294967417.txtar | FIXED | replay clean within left line: E1131/missing, W1002*4 | - | - |
| mutation_E3314-STDLIB-md--4-3--sum-of-an-unknown-list-_4294968792.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain, W1002*4 | - | - |
| mutation_E3322-TYPES-md--5-2--map-key-twice-_4294968344.txtar | FIXED | replay clean | - | - |
| mutation_E3401-TYPES-md--2--optional-of-an-optional-_4294967425.txtar | FIXED | replay clean | - | - |
| mutation_E3401-TYPES-md--2--optional-of-an-optional-_4294967426.txtar | FIXED | replay clean | - | - |
| mutation_E3401-TYPES-md--2--optional-of-an-optional-_4294968286.txtar | FIXED | replay clean | - | - |
| mutation_E3402-TYPES-md--6-5--member-of-an-optional-_4294968801.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain*2, W1002*4 | - | - |
| mutation_E3504-TYPES-md--10-2--ref-to-a-const-_4294967431.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E3504-TYPES-md--10-2--ref-to-a-const-_4294968112.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E3504-TYPES-md--10-2--ref-to-a-const-_4294968793.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E3604-TYPES-md--12-6--match-on-an-Int-_4294968125.txtar | FIXED | replay clean within left line: E1116/token, E1117*2, E2102/plain, E3019, W1002*4 | - | - |
| mutation_E7114-WIRE-md--5-7--table-key-not-an-identifier-_4294967489.txtar | STILL FAILING | expected exactly the operator code; got extra E8009/missing, E8019, W1640 | gen | C: E8019 data-mode emit cascades beside E8018/E7114 |
| mutation_E8009-CODEGEN-md--2-1--invalid-mode-_4294967950.txtar | FIXED | replay clean | - | - |
| mutation_E8011-CODEGEN-md--3-5---go-name---not-an-identifier-_4294967725.txtar | FIXED | replay clean within left line: E8009/missing | - | - |
| mutation_E8011-CODEGEN-md--3-5---go-name---not-an-identifier-_4294968303.txtar | FIXED | replay clean | - | - |
| mutation_E8017-CODEGEN-md--5-6--list-branch-_4294967504.txtar | FIXED | replay clean | - | - |
| mutation_E8018-CODEGEN-md--2-8--data-mode-decodes-a-type-of-a-baked-package-_4294967521.txtar | STILL FAILING | expected exactly the operator code; got extra E8019 | gen | C: E8019 data-mode emit cascades beside E8018/E7114 |
