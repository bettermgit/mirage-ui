import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import RuleSourceCard from '@/pages/subscription/RuleSourceCard';
import { renderWithProviders } from './test-utils';

type Doc = Record<string, unknown>;

const onPatch = vi.fn();

function renderCard(doc: Doc) {
  return renderWithProviders(<RuleSourceCard doc={doc} onPatch={onPatch} />);
}

function openTab(name: RegExp) {
  fireEvent.click(screen.getByRole('tab', { name }));
}

function chip(name: string): HTMLButtonElement {
  return screen.getByRole('button', { name }) as HTMLButtonElement;
}

beforeEach(() => vi.clearAllMocks());

describe('RuleSourceCard builtin rules', () => {
  it('marks the balanced preset chips selected and counts them', () => {
    renderCard({});
    expect(chip('Google').getAttribute('aria-pressed')).toBe('true');
    expect(chip('YouTube').getAttribute('aria-pressed')).toBe('false');
    expect(screen.getByText('14 / 45 enabled')).toBeTruthy();
  });

  it('clicking a chip switches to custom with the toggled set', () => {
    renderCard({});
    fireEvent.click(chip('YouTube'));
    expect(onPatch).toHaveBeenCalledWith(
      expect.objectContaining({
        rule_mode: 'custom',
        enabled_rules: expect.arrayContaining(['youtube', 'google', 'netflix']),
      }),
    );
  });

  it('preset switch drops the custom set', () => {
    renderCard({ rule_mode: 'custom', enabled_rules: ['private'] });
    fireEvent.click(screen.getByLabelText('Full'));
    expect(onPatch).toHaveBeenCalledWith(
      expect.objectContaining({ rule_mode: 'full', enabled_rules: undefined }),
    );
  });

  it('expands the remaining chips beyond the first twelve', () => {
    renderCard({ rule_mode: 'custom', enabled_rules: [] });
    expect(screen.queryByText('PT Sites')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Expand All' }));
    expect(screen.getByText('PT Sites')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Collapse' }));
    expect(screen.queryByText('PT Sites')).toBeNull();
  });
});

describe('RuleSourceCard custom rules', () => {
  const docWithRule: Doc = {
    rule_mode: 'custom',
    enabled_rules: ['private'],
    custom_rules: [
      {
        key: 'ai_rules',
        label: 'AI Rules',
        enabled: true,
        target_mode: 'new_group',
        source_type: 'http',
        behavior: 'domain',
        format: 'mrs',
        url: 'https://e.com/ai.mrs',
        interval: 86400,
        insert_position: 'before_match',
      },
    ],
  };

  it('lists custom rules and toggles them through onPatch', async () => {
    renderCard(docWithRule);
    openTab(/Custom Rules/);
    const row = await screen.findByText('AI Rules');
    expect(row).toBeTruthy();
    const [rowSwitch] = screen.getAllByRole('switch');
    fireEvent.click(rowSwitch);
    expect(onPatch).toHaveBeenCalledWith({
      custom_rules: [expect.objectContaining({ key: 'ai_rules', enabled: false })],
    });
  });

  it('removes a custom rule through the confirm popover', async () => {
    renderCard(docWithRule);
    openTab(/Custom Rules/);
    await screen.findByText('AI Rules');
    fireEvent.click(screen.getByRole('button', { name: 'Remove custom rule' }));
    await screen.findByText('Remove this custom rule?');
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(onPatch).toHaveBeenCalledWith({ custom_rules: [] });
  });

  it('quick-adds remote rule sets with inferred engine shape', async () => {
    renderCard({ rule_mode: 'custom', enabled_rules: [] });
    openTab(/Custom Rules/);
    fireEvent.change(screen.getByLabelText('Rule URLs'), {
      target: { value: 'AI https://example.com/rules/ai.mrs' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add & Save' }));
    await waitFor(() => expect(onPatch).toHaveBeenCalled());
    const patch = onPatch.mock.calls[0][0] as { custom_rules: Record<string, unknown>[] };
    expect(patch.custom_rules[0]).toMatchObject({
      key: 'ai_2',
      label: 'AI',
      source_type: 'http',
      behavior: 'domain',
      format: 'mrs',
      interval: 86400,
    });
  });

  it('normalizes direct rules on blur and writes additional_rules', () => {
    renderCard({});
    openTab(/Custom Rules/);
    const box = screen.getByLabelText('Mihomo Rules');
    fireEvent.change(box, { target: { value: '- DOMAIN-SUFFIX,pp.ua,DIRECT # note' } });
    fireEvent.blur(box);
    expect(onPatch).toHaveBeenCalledWith({ additional_rules: ['DOMAIN-SUFFIX,pp.ua,DIRECT'] });
  });

  it('opens the add-rule modal and appends a validated inline rule', async () => {
    renderCard({ rule_mode: 'custom', enabled_rules: [] });
    openTab(/Custom Rules/);
    fireEvent.click(screen.getByRole('button', { name: 'Add Rule' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'My Rules' } });
    fireEvent.change(within(dialog).getByLabelText('Inline Rules'), {
      target: { value: 'domain,ads.example.com\nnetwork,10.0.0.0/8' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(onPatch).toHaveBeenCalled());
    const patch = onPatch.mock.calls[0][0] as { custom_rules: Record<string, unknown>[] };
    expect(patch.custom_rules[0]).toMatchObject({
      key: 'my_rules',
      label: 'My Rules',
      source_type: 'inline',
      payload: ['domain,ads.example.com', 'network,10.0.0.0/8'],
    });
  });
});

describe('RuleSourceCard template mode', () => {
  it('switching to a catalog preset writes external_config', async () => {
    renderCard({ template_rule_mode: 'template', template: 'standard' });
    const combo = screen.getByRole('combobox', { name: 'Template Preset' });
    fireEvent.mouseDown(combo);
    fireEvent.click(await screen.findByText('ACL4SSR Online'));
    expect(onPatch).toHaveBeenCalledWith({
      external_config: { template_key: 'acl4ssr_online', template_label: 'ACL4SSR Online' },
    });
  });

  it('mode segmented maps to template_rule_mode with default dropped', () => {
    renderCard({});
    fireEvent.click(screen.getByLabelText('Template Mode'));
    expect(onPatch).toHaveBeenCalledWith(
      expect.objectContaining({ template_rule_mode: 'template' }),
    );
  });
});
