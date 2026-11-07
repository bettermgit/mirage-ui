import { describe, expect, it } from 'vitest';

import {
  RULE_CATALOG_KEYS,
  RULE_PRESETS,
  TEMPLATE_PRESETS,
  buildQuickAddRules,
  enabledRuleSet,
  normalizeDirectRuleLines,
  parseQuickAddLines,
  slugRuleKey,
} from '@/pages/subscription/ruleCatalog';

describe('rule catalog parity with the vendored engine', () => {
  it('lists every effective engine category and drops the dead key', () => {
    expect(RULE_CATALOG_KEYS).toContain('aethersailor_direct');
    // nonstandard_port exists in the Go map but appears in no output order, so
    // enabling it renders nothing; exposing it would be a dead switch.
    expect(RULE_CATALOG_KEYS).not.toContain('nonstandard_port');
    expect(new Set(RULE_CATALOG_KEYS).size).toBe(RULE_CATALOG_KEYS.length);
  });

  it('mirrors the engine preset sets with legacy keys expanded', () => {
    expect(RULE_PRESETS.minimal).toEqual(['private', 'domestic', 'non_cn']);
    expect(RULE_PRESETS.balanced).toContain('google');
    expect(RULE_PRESETS.balanced).toContain('telegram');
    // The engine balanced preset carries legacy "streaming"; the UI must show
    // its expansion instead of the alias.
    expect(RULE_PRESETS.balanced).not.toContain('streaming');
    for (const key of RULE_PRESETS.balanced) expect(RULE_CATALOG_KEYS).toContain(key);
  });

  it('resolves the active chip set from rule_mode like resolveEnabledRules', () => {
    expect([...enabledRuleSet('', [])]).toEqual([...enabledRuleSet('balanced', [])]);
    expect([...enabledRuleSet('minimal', [])]).toEqual(RULE_PRESETS.minimal);
    expect([...enabledRuleSet('full', [])].sort()).toEqual([...RULE_CATALOG_KEYS].sort());
    const custom = enabledRuleSet('custom', ['streaming', 'private', 'bogus', 'private']);
    expect(custom.has('netflix')).toBe(true);
    expect(custom.has('asian_streaming')).toBe(true);
    expect(custom.has('private')).toBe(true);
    expect(custom.has('bogus')).toBe(false);
    expect(custom.has('streaming')).toBe(false);
  });
});

describe('remote rule-set quick add', () => {
  it('parses name/URL lines and counts invalid ones like the subconv console', () => {
    const { entries, invalid } = parseQuickAddLines(
      [
        'https://example.com/rules/ai.mrs',
        '社媒规则 https://example.com/rules/social.mrs',
        '游戏 https://example.com/rules/game.mrs # keep',
        '# comment only',
        'not a url at all',
        'ftp://example.com/x.mrs',
      ].join('\n'),
      '',
    );
    expect(invalid).toBe(2);
    expect(entries.map((e) => e.name)).toEqual(['ai', '社媒规则', '游戏']);
    expect(entries[1].url).toBe('https://example.com/rules/social.mrs');
  });

  it('applies the single-name override only to one untyped line', () => {
    expect(parseQuickAddLines('https://e.com/a.mrs', '我的规则集').entries[0].name).toBe(
      '我的规则集',
    );
    const two = parseQuickAddLines('https://e.com/a.mrs\nhttps://e.com/b.mrs', '我的规则集');
    expect(two.entries.map((e) => e.name)).toEqual(['a', 'b']);
  });

  it('builds engine-compatible custom_rules entries with unique keys', () => {
    const added = buildQuickAddRules(
      [
        { name: 'AI Rules', url: 'https://e.com/ai.mrs' },
        { name: 'AI Rules', url: 'https://e.com/ai2.mrs' },
        { name: '社媒', url: 'https://e.com/social.mrs' },
      ],
      new Set(['ai_rules']),
    );
    expect(added.map((r) => r.key)).toEqual(['ai_rules_2', 'ai_rules_3', 'social']);
    expect(added[0]).toMatchObject({
      label: 'AI Rules',
      enabled: true,
      target_mode: 'new_group',
      source_type: 'http',
      behavior: 'domain',
      format: 'mrs',
      interval: 86400,
      insert_position: 'before_match',
    });
    expect(added[2]).toMatchObject({ label: '社媒', format: 'mrs' });
  });

  it('infers format from the URL and classical behavior from known repos', () => {
    const added = buildQuickAddRules(
      [
        { name: 'Y', url: 'https://e.com/r.yaml' },
        { name: 'L', url: 'https://e.com/r.list' },
        {
          name: 'BM7',
          url: 'https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Global/Global.yaml',
        },
      ],
      new Set(),
    );
    expect(added[0]).toMatchObject({ behavior: 'domain', format: 'yaml' });
    expect(added[1]).toMatchObject({ behavior: 'domain', format: 'text' });
    expect(added[2]).toMatchObject({ behavior: 'classical', format: 'yaml' });
  });
});

describe('direct rules textarea', () => {
  it('normalizes pasted mihomo rule lines', () => {
    const lines = normalizeDirectRuleLines(
      [
        '- DOMAIN-KEYWORD,tracker,PROXY',
        '  # commented out',
        'IP-CIDR,10.0.0.6/32,PROXY # inline note',
        '"DOMAIN-SUFFIX,pp.ua,DIRECT"',
        'DOMAIN-KEYWORD,tracker,PROXY',
        '',
      ].join('\n'),
    );
    expect(lines).toEqual([
      'DOMAIN-KEYWORD,tracker,PROXY',
      'IP-CIDR,10.0.0.6/32,PROXY',
      'DOMAIN-SUFFIX,pp.ua,DIRECT',
    ]);
  });
});

describe('custom rule keys', () => {
  it('slugs ascii labels and gives up on CJK', () => {
    expect(slugRuleKey('  My AI Rules!! ')).toBe('my_ai_rules');
    expect(slugRuleKey('社媒规则')).toBe('');
  });
});

describe('template catalog', () => {
  it('carries the vendored templatecatalog presets', () => {
    const keys = TEMPLATE_PRESETS.map((t) => t.key);
    expect(keys).toContain('subconv_standard');
    expect(keys).toContain('cm_online_full_cf');
    expect(keys).toContain('acl4ssr_online_adblock');
    expect(keys).toContain('blackmatrix7_global');
    for (const preset of TEMPLATE_PRESETS) {
      expect(preset.group).toBeTruthy();
      expect(preset.label).toBeTruthy();
    }
  });
});
