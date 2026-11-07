import type { ReactNode } from 'react';

import {
  AndroidOutlined,
  ApiOutlined,
  AppleOutlined,
  BookOutlined,
  CameraOutlined,
  CloudOutlined,
  CodeOutlined,
  CodeSandboxOutlined,
  ControlOutlined,
  CrownOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DesktopOutlined,
  DislikeOutlined,
  DiscordOutlined,
  ExperimentOutlined,
  FireOutlined,
  GithubOutlined,
  GlobalOutlined,
  GoogleOutlined,
  HddOutlined,
  HomeOutlined,
  LockOutlined,
  MessageOutlined,
  NotificationOutlined,
  PayCircleOutlined,
  PlayCircleOutlined,
  ProfileOutlined,
  RocketOutlined,
  SafetyOutlined,
  SendOutlined,
  ShareAltOutlined,
  ShoppingOutlined,
  SkinOutlined,
  SoundOutlined,
  StopOutlined,
  SwapOutlined,
  CloudServerOutlined,
  TeamOutlined,
  TwitterOutlined,
  UsbOutlined,
  VideoCameraOutlined,
  WalletOutlined,
  YoutubeOutlined,
} from '@ant-design/icons';

export interface RuleCatalogEntry {
  key: string;
  icon: ReactNode;
}

// Catalog = vendored engine's ruleCatalog map in ruleOutputOrder, minus
// nonstandard_port which sits in no output order and renders nothing.
export const RULE_CATALOG: RuleCatalogEntry[] = [
  { key: 'adblock', icon: <StopOutlined aria-hidden /> },
  { key: 'ai', icon: <ExperimentOutlined aria-hidden /> },
  { key: 'youtube', icon: <YoutubeOutlined aria-hidden /> },
  { key: 'education', icon: <BookOutlined aria-hidden /> },
  { key: 'cloud', icon: <CloudOutlined aria-hidden /> },
  { key: 'google_fcm', icon: <NotificationOutlined aria-hidden /> },
  { key: 'google', icon: <GoogleOutlined aria-hidden /> },
  { key: 'private', icon: <LockOutlined aria-hidden /> },
  { key: 'domestic', icon: <HomeOutlined aria-hidden /> },
  { key: 'telegram', icon: <SendOutlined aria-hidden /> },
  { key: 'github', icon: <GithubOutlined aria-hidden /> },
  { key: 'speedtest', icon: <DashboardOutlined aria-hidden /> },
  { key: 'onedrive', icon: <HddOutlined aria-hidden /> },
  { key: 'microsoft', icon: <DesktopOutlined aria-hidden /> },
  { key: 'apple', icon: <AppleOutlined aria-hidden /> },
  { key: 'twitter', icon: <TwitterOutlined aria-hidden /> },
  { key: 'meta', icon: <TeamOutlined aria-hidden /> },
  { key: 'discord', icon: <DiscordOutlined aria-hidden /> },
  { key: 'social', icon: <ShareAltOutlined aria-hidden /> },
  { key: 'communication', icon: <MessageOutlined aria-hidden /> },
  { key: 'social_media', icon: <SoundOutlined aria-hidden /> },
  { key: 'netflix', icon: <FireOutlined aria-hidden /> },
  { key: 'disney', icon: <PlayCircleOutlined aria-hidden /> },
  { key: 'western_streaming', icon: <VideoCameraOutlined aria-hidden /> },
  { key: 'asian_streaming', icon: <CameraOutlined aria-hidden /> },
  { key: 'emby', icon: <CloudServerOutlined aria-hidden /> },
  { key: 'steam', icon: <SkinOutlined aria-hidden /> },
  { key: 'pc_gaming', icon: <CodeSandboxOutlined aria-hidden /> },
  { key: 'console_gaming', icon: <CrownOutlined aria-hidden /> },
  { key: 'game_platforms', icon: <ControlOutlined aria-hidden /> },
  { key: 'dev', icon: <CodeOutlined aria-hidden /> },
  { key: 'containers', icon: <UsbOutlined aria-hidden /> },
  { key: 'storage', icon: <DatabaseOutlined aria-hidden /> },
  { key: 'payment', icon: <PayCircleOutlined aria-hidden /> },
  { key: 'crypto', icon: <WalletOutlined aria-hidden /> },
  { key: 'news', icon: <ProfileOutlined aria-hidden /> },
  { key: 'shopping', icon: <ShoppingOutlined aria-hidden /> },
  { key: 'ai_cn', icon: <AndroidOutlined aria-hidden /> },
  { key: 'ai_extended', icon: <ApiOutlined aria-hidden /> },
  { key: 'aethersailor_direct', icon: <RocketOutlined aria-hidden /> },
  { key: 'game_direct', icon: <FireOutlined aria-hidden /> },
  { key: 'entertainment', icon: <DislikeOutlined aria-hidden /> },
  { key: 'pt', icon: <SwapOutlined aria-hidden /> },
  { key: 'gfw', icon: <SafetyOutlined aria-hidden /> },
  { key: 'non_cn', icon: <GlobalOutlined aria-hidden /> },
];

export const RULE_CATALOG_KEYS = RULE_CATALOG.map((entry) => entry.key);

// Aliases the engine expands in normalizeEnabledRuleKeys.
export const LEGACY_RULE_EXPANSIONS: Record<string, string[]> = {
  streaming: ['netflix', 'disney', 'western_streaming', 'asian_streaming'],
  gaming: ['steam', 'pc_gaming', 'console_gaming'],
  finance: ['payment', 'crypto'],
  social: ['twitter', 'meta', 'discord', 'social'],
  bilibili: ['asian_streaming'],
};

const PRESETS_RAW: Record<'minimal' | 'balanced', string[]> = {
  minimal: ['private', 'domestic', 'non_cn'],
  balanced: [
    'private',
    'domestic',
    'onedrive',
    'microsoft',
    'apple',
    'google',
    'github',
    'ai',
    'telegram',
    'streaming',
    'non_cn',
  ],
};

export const RULE_PRESETS: Record<'minimal' | 'balanced', string[]> = {
  minimal: expandLegacyRules(PRESETS_RAW.minimal),
  balanced: expandLegacyRules(PRESETS_RAW.balanced),
};

export function expandLegacyRules(keys: string[]): string[] {
  const catalog = new Set(RULE_CATALOG_KEYS);
  const out: string[] = [];
  const seen = new Set<string>();
  const visit = (raw: string, depth: number) => {
    const key = String(raw ?? '')
      .trim()
      .toLowerCase();
    if (!key || depth > 4) return;
    const legacy = LEGACY_RULE_EXPANSIONS[key];
    if (legacy) {
      for (const item of legacy) visit(item, depth + 1);
      return;
    }
    if (!catalog.has(key) || seen.has(key)) return;
    seen.add(key);
    out.push(key);
  };
  for (const key of keys) visit(key, 0);
  return out;
}

export function enabledRuleSet(ruleMode: string, enabledRules: string[]): Set<string> {
  const mode = ruleMode.trim().toLowerCase();
  if (mode === 'full') return new Set(RULE_CATALOG_KEYS);
  if (mode === 'minimal') return new Set(RULE_PRESETS.minimal);
  if (mode === 'balanced' || mode === '') return new Set(RULE_PRESETS.balanced);
  return new Set(expandLegacyRules(enabledRules));
}

export interface TemplatePreset {
  key: string;
  group: string;
  label: string;
}

// Mirror of internal/subconvengine/templatecatalog presetList keys.
export const TEMPLATE_PRESETS: TemplatePreset[] = [
  { key: 'subconv_lite', group: 'SubConv Next', label: 'SubConv Next Lite' },
  { key: 'subconv_standard', group: 'SubConv Next', label: 'SubConv Next Standard' },
  { key: 'subconv_full', group: 'SubConv Next', label: 'SubConv Next Full' },
  { key: 'cm_online', group: 'CM Online', label: 'CM Online' },
  { key: 'cm_online_game', group: 'CM Online', label: 'CM Online + Game' },
  { key: 'cm_online_multi_country', group: 'CM Online', label: 'CM Online Multi Country' },
  { key: 'cm_online_multi_country_cf', group: 'CM Online', label: 'CM Online Multi Country CF' },
  { key: 'cm_online_full', group: 'CM Online', label: 'CM Online Full' },
  { key: 'cm_online_full_cf', group: 'CM Online', label: 'CM Online Full CF' },
  { key: 'acl4ssr_online_mini', group: 'ACL4SSR', label: 'ACL4SSR Online Mini' },
  { key: 'acl4ssr_online', group: 'ACL4SSR', label: 'ACL4SSR Online' },
  { key: 'acl4ssr_online_adblock', group: 'ACL4SSR', label: 'ACL4SSR Online Adblocked' },
  { key: 'acl4ssr_online_full', group: 'ACL4SSR', label: 'ACL4SSR Online Full' },
  { key: 'blackmatrix7_basic', group: 'blackmatrix7', label: 'blackmatrix7 Basic' },
  { key: 'blackmatrix7_streaming', group: 'blackmatrix7', label: 'blackmatrix7 Streaming' },
  { key: 'blackmatrix7_global', group: 'blackmatrix7', label: 'blackmatrix7 Global' },
];

export interface CustomRuleEntry {
  key: string;
  label: string;
  icon?: string;
  enabled: boolean;
  target_mode: string;
  target_group?: string;
  source_type: string;
  behavior?: string;
  format?: string;
  url?: string;
  path?: string;
  interval?: number;
  payload?: string[];
  insert_position?: string;
  no_resolve?: boolean;
}

export interface QuickAddEntry {
  name: string;
  url: string;
}

export function parseQuickAddLines(
  text: string,
  singleName: string,
): { entries: QuickAddEntry[]; invalid: number } {
  const lines = String(text ?? '')
    .split(/\r?\n/)
    .map((line) => {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) return '';
      const comment = trimmed.indexOf(' #');
      return comment >= 0 ? trimmed.slice(0, comment).trim() : trimmed;
    })
    .filter(Boolean);
  const entries: QuickAddEntry[] = [];
  let invalid = 0;
  lines.forEach((line, index) => {
    const parts = line.split(/\s+/);
    let name = '';
    let url = '';
    if (parts.length === 1) {
      url = parts[0];
    } else if (/^https?:\/\//i.test(parts[parts.length - 1])) {
      name = parts.slice(0, -1).join(' ');
      url = parts[parts.length - 1];
    } else {
      invalid += 1;
      return;
    }
    let parsed: URL;
    try {
      parsed = new URL(url);
    } catch {
      invalid += 1;
      return;
    }
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      invalid += 1;
      return;
    }
    if (!name && index === 0 && lines.length === 1) name = singleName.trim();
    if (!name) name = ruleNameFromURL(parsed.href) || `rule_${index + 1}`;
    entries.push({ name, url: parsed.href });
  });
  return { entries, invalid };
}

export function ruleNameFromURL(rawURL: string): string {
  try {
    const parsed = new URL(String(rawURL ?? '').trim());
    const segments = parsed.pathname.split('/').filter(Boolean);
    const base = segments.length ? segments[segments.length - 1] : '';
    return base.replace(/\.(mrs|list|txt|yaml|yml)$/i, '').trim() || parsed.hostname;
  } catch {
    return '';
  }
}

export function slugRuleKey(value: string): string {
  return String(value ?? '')
    .trim()
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/_+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 64);
}

function uniqueCustomRuleKey(baseKey: string, taken: Set<string>): string {
  const base = baseKey || 'custom_rule';
  if (!taken.has(base)) return base;
  for (let n = 2; ; n += 1) {
    const candidate = `${base}_${n}`.slice(0, 64);
    if (!taken.has(candidate)) return candidate;
  }
}

// Mirrors the engine's normalizeRemoteRuleProviderShape for URL sources.
export function inferRemoteRuleShape(rawURL: string): { behavior: string; format: string } {
  const source = String(rawURL ?? '')
    .toLowerCase()
    .trim();
  let format = '';
  if (source.endsWith('.mrs')) format = 'mrs';
  else if (source.endsWith('.yaml') || source.endsWith('.yml')) format = 'yaml';
  else if (source.endsWith('.txt') || source.endsWith('.text')) format = 'text';
  let behavior = 'domain';
  if (source.includes('/rule/clash/') || source.includes('blackmatrix7/ios_rule_script')) {
    behavior = 'classical';
    format = 'yaml';
  }
  if (!format) format = 'text';
  if (behavior === 'classical' && format === 'mrs') format = 'yaml';
  return { behavior, format };
}

export function buildQuickAddRules(
  entries: QuickAddEntry[],
  takenKeys: Set<string>,
): CustomRuleEntry[] {
  const taken = new Set(takenKeys);
  return entries.map((entry) => {
    let key = slugRuleKey(entry.name);
    if (!key) key = slugRuleKey(ruleNameFromURL(entry.url));
    if (!key) key = `mrs_${Date.now().toString(36)}`;
    key = uniqueCustomRuleKey(key, taken);
    taken.add(key);
    const { behavior, format } = inferRemoteRuleShape(entry.url);
    return {
      key,
      label: entry.name,
      icon: '',
      enabled: true,
      target_mode: 'new_group',
      target_group: '',
      source_type: 'http',
      behavior,
      format,
      url: entry.url,
      path: '',
      interval: 86400,
      payload: [],
      insert_position: 'before_match',
      no_resolve: false,
    };
  });
}

export function normalizeDirectRuleLines(value: string): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const raw of String(value ?? '').split(/\r?\n/)) {
    let line = raw.trim();
    if (!line || line.startsWith('#')) continue;
    line = line
      .replace(/^-\s*/, '')
      .replace(/^['"]|['"]$/g, '')
      .trim();
    if (!line || line.startsWith('#')) continue;
    const comment = line.indexOf(' #');
    if (comment >= 0) line = line.slice(0, comment).trim();
    if (!line || seen.has(line)) continue;
    seen.add(line);
    out.push(line);
  }
  return out;
}
