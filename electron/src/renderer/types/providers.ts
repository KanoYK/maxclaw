export interface ModelConfig {
  id: string;
  name: string;
  maxTokens?: number;
  enabled: boolean;
  supportsImageInput?: boolean;
}

export const SUBSCRIPTION_PROVIDERS = {
  'codex-cli': {
    name: 'Codex (ChatGPT)',
    command: 'codex login',
    installURL: 'https://learn.chatgpt.com/docs/codex/cli',
  },
  'claude-code': {
    name: 'Claude Code',
    command: 'claude auth login',
    installURL: 'https://code.claude.com/docs/en/quickstart',
  },
} as const;

export function isSubscriptionFormat(format?: string): format is keyof typeof SUBSCRIPTION_PROVIDERS {
  return format === 'codex-cli' || format === 'claude-code';
}

export interface ProviderConfig {
  id: string;
  name: string;
  type: 'openai' | 'anthropic' | 'custom';
  apiKey: string;
  baseURL?: string;
  apiFormat: 'openai' | 'anthropic' | keyof typeof SUBSCRIPTION_PROVIDERS;
  models: ModelConfig[];
  enabled: boolean;
}

export const PRESET_PROVIDERS: Omit<ProviderConfig, 'id' | 'apiKey'>[] = [
  {
    name: SUBSCRIPTION_PROVIDERS['codex-cli'].name,
    type: 'openai',
    apiFormat: 'codex-cli',
    models: [{ id: 'default', name: 'Codex (ChatGPT)', enabled: true }],
    enabled: false,
  },
  {
    name: SUBSCRIPTION_PROVIDERS['claude-code'].name,
    type: 'anthropic',
    apiFormat: 'claude-code',
    models: [{ id: 'default', name: 'Claude Code', enabled: true }],
    enabled: false,
  },
  {
    name: 'OpenRouter',
    type: 'openai',
    baseURL: 'https://openrouter.ai/api/v1',
    apiFormat: 'openai',
    models: [
      { id: 'openrouter/auto', name: 'OpenRouter Auto', enabled: true },
      { id: 'openrouter/anthropic/claude-sonnet-4.5', name: 'Claude Sonnet 4.5', enabled: true, supportsImageInput: true },
    ],
    enabled: false,
  },
  {
    name: 'DeepSeek',
    type: 'openai',
    baseURL: 'https://api.deepseek.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'deepseek-chat', name: 'DeepSeek Chat', enabled: true, supportsImageInput: false },
      { id: 'deepseek-coder', name: 'DeepSeek Coder', enabled: true, supportsImageInput: false },
    ],
    enabled: false,
  },
  {
    name: 'OpenAI',
    type: 'openai',
    baseURL: 'https://api.openai.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'gpt-5.1', name: 'GPT-5.1', enabled: true, supportsImageInput: true },
      { id: 'gpt-5-mini', name: 'GPT-5 mini', enabled: true, supportsImageInput: true },
      { id: 'gpt-4.1-mini', name: 'GPT-4.1 mini', enabled: true, supportsImageInput: true },
    ],
    enabled: false,
  },
  {
    name: 'Anthropic',
    type: 'anthropic',
    baseURL: 'https://api.anthropic.com',
    apiFormat: 'anthropic',
    models: [
      { id: 'claude-opus-4-1', name: 'Claude Opus 4.1', enabled: true, supportsImageInput: true },
      { id: 'claude-sonnet-4', name: 'Claude Sonnet 4', enabled: true, supportsImageInput: true },
    ],
    enabled: false,
  },
  {
    name: 'Zhipu',
    type: 'openai',
    baseURL: 'https://open.bigmodel.cn/api/coding/paas/v4',
    apiFormat: 'openai',
    models: [
      { id: 'glm-5', name: 'GLM-5', enabled: true },
      { id: 'glm-4.7', name: 'GLM-4.7', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Xiaomi',
    type: 'openai',
    baseURL: 'https://api.xiaomimimo.com/v1',
    apiFormat: 'openai',
    models: [
      { id: 'mimo-v2.5-pro', name: 'MiMo V2.5 Pro', enabled: true, supportsImageInput: true },
      { id: 'mimo-v2.5', name: 'MiMo V2.5', enabled: true, supportsImageInput: true },
      { id: 'mimo-v2-omni', name: 'MiMo V2 Omni', enabled: true, supportsImageInput: true },
    ],
    enabled: false,
  },
  {
    name: 'Moonshot',
    type: 'openai',
    baseURL: 'https://api.moonshot.cn/v1',
    apiFormat: 'openai',
    models: [
      { id: 'kimi-k2-0905-preview', name: 'Kimi K2 0905 Preview', enabled: true },
      { id: 'kimi-k2-turbo-preview', name: 'Kimi K2 Turbo Preview', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Groq',
    type: 'openai',
    baseURL: 'https://api.groq.com/openai/v1',
    apiFormat: 'openai',
    models: [
      { id: 'llama-3.3-70b-versatile', name: 'Llama 3.3 70B', enabled: true },
      { id: 'mistral-saba-24b', name: 'Mistral Saba 24B', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'Gemini',
    type: 'openai',
    baseURL: 'https://generativelanguage.googleapis.com/v1beta',
    apiFormat: 'openai',
    models: [
      { id: 'gemini-2.5-pro', name: 'Gemini 2.5 Pro', enabled: true, supportsImageInput: true },
      { id: 'gemini-2.5-flash', name: 'Gemini 2.5 Flash', enabled: true, supportsImageInput: true },
    ],
    enabled: false,
  },
  {
    name: 'DashScope',
    type: 'openai',
    baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    apiFormat: 'openai',
    models: [
      { id: 'qwen-max-latest', name: 'Qwen Max Latest', enabled: true },
      { id: 'qwen-plus-latest', name: 'Qwen Plus Latest', enabled: true },
    ],
    enabled: false,
  },
  {
    name: 'MiniMax',
    type: 'openai',
    baseURL: 'https://api.minimax.io/v1',
    apiFormat: 'openai',
    models: [
      { id: 'MiniMax-M2.5', name: 'MiniMax M2.5', enabled: true },
      { id: 'MiniMax-M2.5-highspeed', name: 'MiniMax M2.5 Highspeed', enabled: true },
    ],
    enabled: false,
  },
];
