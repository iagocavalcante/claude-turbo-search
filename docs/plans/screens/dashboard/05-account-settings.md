# Tela: Configuracoes da Conta

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** MVP
**Tecnologia:** Phoenix LiveView

---

## Objetivo

Gerenciar informacoes da empresa, seguranca e configuracoes do servico.

---

## Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo] Phone Validator          [Docs]  [Suporte]  [User ▼]       │
│──────────────────────────────────────────────────────────────────────│
│  │                                                                  │
│  │ Dashboard          │  Configuracoes                              │
│  │ API Keys           │                                             │
│  │ Logs               │  [Geral] [Seguranca] [Webhooks] [Equipe]   │
│  │ Configuracoes ←    │                                             │
│  │                    │                                             │
└──┴────────────────────┴─────────────────────────────────────────────┘
```

---

## Tab: Geral

```
│  INFORMACOES DA EMPRESA                                    │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Nome da Empresa                                     │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ Empresa XYZ Ltda                         │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Email de Contato                                    │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ admin@empresaxyz.com                     │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  CNPJ (opcional)                                     │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ 12.345.678/0001-90                       │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  ┌────────────┐                                      │  │
│  │  │  Salvar    │                                      │  │
│  │  └────────────┘                                      │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  PLANO ATUAL                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Plano: Free                                         │  │
│  │  Validacoes/mes: 100 de 500 usadas                   │  │
│  │  ████████░░░░░░░░░░░░  20%                           │  │
│  │                                                      │  │
│  │  [Fazer Upgrade]                                     │  │
│  └──────────────────────────────────────────────────────┘  │
```

---

## Tab: Seguranca

```
│  ALTERAR SENHA                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Senha Atual                                         │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ ••••••••                                 │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Nova Senha                                          │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │                                          │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Confirmar Nova Senha                                │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │                                          │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  ┌──────────────────┐                                │  │
│  │  │  Alterar Senha   │                                │  │
│  │  └──────────────────┘                                │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  IP WHITELIST                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Restrinja o acesso a API a IPs especificos.         │  │
│  │  Deixe vazio para permitir qualquer IP.              │  │
│  │                                                      │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ 189.100.50.1                         [X] │        │  │
│  │  │ 200.150.75.0/24                      [X] │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ Adicionar IP ou CIDR...                  │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │  [+ Adicionar]                                       │  │
│  │                                                      │  │
│  │  ┌────────────┐                                      │  │
│  │  │  Salvar    │                                      │  │
│  │  └────────────┘                                      │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  SESSOES ATIVAS                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Chrome - macOS (atual)                              │  │
│  │  IP: 189.100.50.1 - Ultimo acesso: Agora             │  │
│  │                                                      │  │
│  │  Firefox - Windows                                   │  │
│  │  IP: 200.150.75.10 - Ultimo acesso: 2h atras        │  │
│  │  [Encerrar]                                          │  │
│  └──────────────────────────────────────────────────────┘  │
```

---

## Tab: Webhooks

```
│  WEBHOOK DE CALLBACK                                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Receba notificacoes quando validacoes forem          │  │
│  │  confirmadas.                                        │  │
│  │                                                      │  │
│  │  URL do Webhook                                      │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ https://api.minhaempresa.com/webhooks    │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Secret (para verificar assinatura)                  │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ whsec_xxxxxxxxxxxxxxxx       [Regenerar] │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Eventos:                                            │  │
│  │  [✓] validation.confirmed                            │  │
│  │  [✓] validation.expired                              │  │
│  │  [ ] validation.failed                               │  │
│  │                                                      │  │
│  │  ┌────────────┐  ┌───────────────────┐               │  │
│  │  │  Salvar    │  │  Enviar Teste     │               │  │
│  │  └────────────┘  └───────────────────┘               │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  HISTORICO DE WEBHOOKS                                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  14:32  validation.confirmed   200 OK     45ms       │  │
│  │  14:28  validation.expired     200 OK     52ms       │  │
│  │  11:15  validation.confirmed   500 Error  120ms      │  │
│  │         ↳ Retry 1: 200 OK     48ms                   │  │
│  └──────────────────────────────────────────────────────┘  │
```

---

## Tab: Equipe

```
│  MEMBROS DA EQUIPE                                         │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  ┌──────────────────────────────────────────────┐    │  │
│  │  │ Joao Silva        admin@empresa.com   Admin  │    │  │
│  │  │ (voce)                                       │    │  │
│  │  ├──────────────────────────────────────────────┤    │  │
│  │  │ Maria Santos      maria@empresa.com  Viewer  │    │  │
│  │  │                               [Editar] [X]   │    │  │
│  │  └──────────────────────────────────────────────┘    │  │
│  │                                                      │  │
│  │  ┌────────────────────────┐                          │  │
│  │  │  + Convidar Membro    │                          │  │
│  │  └────────────────────────┘                          │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  PAPEIS                                                    │
│  Admin: Acesso total                                       │
│  Developer: API Keys + Logs                                │
│  Viewer: Apenas visualizacao                               │
```

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Salvar (qualquer secao) | Salva alteracoes, toast "Salvo com sucesso" |
| Alterar senha (valido) | Atualiza senha, invalida outras sessoes |
| Adicionar IP whitelist | Valida formato IP/CIDR, adiciona a lista |
| Remover IP whitelist | Remove da lista (salvar para confirmar) |
| Enviar Teste (webhook) | Envia payload de teste para URL configurada |
| Regenerar Secret | Novo secret gerado, aviso de impacto |
| Convidar membro | Modal com email + papel → Email de convite |
| Encerrar sessao | Remove sessao selecionada |

---

## Validacoes

| Campo | Regra |
|-------|-------|
| Nome da Empresa | Min 2 chars |
| Email | Formato valido |
| CNPJ | Formato valido (XX.XXX.XXX/XXXX-XX) |
| Senha | Min 8 chars, 1 numero, 1 maiuscula |
| IP Whitelist | IPv4, IPv6 ou CIDR valido |
| Webhook URL | URL valida com HTTPS |

---

## Notas Tecnicas

- IP Whitelist salva em `clients.settings["ip_whitelist"]`
- Webhook secret gerado com :crypto.strong_rand_bytes
- Webhook delivery com retry (3 tentativas, backoff exponencial)
- Webhook payload assinado com HMAC-SHA256 usando secret
- Equipe: convite por email com link de ativacao (expira em 48h)
- Papeis armazenados em tabela `team_members` com enum de roles
