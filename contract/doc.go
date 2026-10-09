// Package contract agrupa os compromissos de rede compartilhados pela API
// principal e pelo Monitor: HMAC das rotas internas (Fase 2), token de SSO
// (Fase 3), envelopes de transferência/estado e erros da rede.
//
// Regra do pacote (PLANO_MONITOR_FLUXO_ALUNO.md): só entra o que atravessa a
// rede — regra de negócio, banco e log permanecem em cada serviço.
package contract
