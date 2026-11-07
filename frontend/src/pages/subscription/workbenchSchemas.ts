import { z } from 'zod';

export const SourceFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'pages.subscription.errNameRequired')
    .max(256, 'pages.subscription.errNameRequired'),
  url: z
    .string()
    .trim()
    .min(1, 'pages.subscription.errUrlRequired')
    .refine((v) => v.startsWith('http://') || v.startsWith('https://'), {
      message: 'pages.subscription.errUrlHttp',
    }),
  enable: z.boolean(),
});
export type SourceFormValues = z.infer<typeof SourceFormSchema>;

export const PublicationFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'pages.subscription.errNameRequired')
    .max(256, 'pages.subscription.errNameRequired'),
  sourceIds: z.array(z.string()),
  subIds: z.array(z.string()),
  enable: z.boolean(),
  rotateToken: z.boolean(),
});
export type PublicationFormValues = z.infer<typeof PublicationFormSchema>;

export const CustomRuleFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'pages.subscription.errNameRequired')
    .max(256, 'pages.subscription.errNameRequired'),
  sourceType: z.enum(['inline', 'http', 'group_only']),
  url: z.string().trim(),
  payloadText: z.string(),
  targetMode: z.enum(['new_group', 'direct', 'reject', 'existing_group']),
  targetGroup: z.string(),
  interval: z.number().min(600),
  noResolve: z.boolean(),
});
export type CustomRuleFormValues = z.infer<typeof CustomRuleFormSchema>;
