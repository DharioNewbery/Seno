<script lang="ts">
  import type { PageProps } from "./$types";
  import { ROTULO_CARGO, type Cargo } from "#lib/tipos";

  let { data }: PageProps = $props();

  const metricas = $derived(data.metricas);

  const STATUS_USUARIO: Record<string, string> = {
    active: "Ativo",
    invited: "Convidado",
    disabled: "Desativado",
  };

  type Linha = { chave: string; rotulo: string; qtd: number };

  const capitalizar = (t: string) => t.charAt(0).toUpperCase() + t.slice(1);

  /** Converte um mapa chave → quantidade em linhas rotuladas, da maior para a menor. */
  function linhas(mapa: Record<string, number> | undefined, rotulos: Record<string, string> = {}): Linha[] {
    return Object.entries(mapa ?? {})
      .map(([chave, qtd]) => ({ chave, qtd, rotulo: rotulos[chave] ?? capitalizar(chave) }))
      .sort((a, b) => b.qtd - a.qtd);
  }

  const paineis = $derived(
    metricas
      ? [
          { titulo: "Usuários por cargo", linhas: linhas(metricas.usuarios_por_cargo, ROTULO_CARGO as Record<Cargo, string>), vazio: "Sem dados." },
          { titulo: "Usuários por situação", linhas: linhas(metricas.usuarios_por_status, STATUS_USUARIO), vazio: "Sem dados." },
          { titulo: "Correções por situação", linhas: linhas(metricas.submissoes_por_status), vazio: "Sem dados." },
          { titulo: "Fila de execução", linhas: linhas(metricas.fila_judge0), vazio: "Fila vazia: nenhuma execução pendente." },
        ]
      : [],
  );
</script>

<section class="admin-dashboard">
  {#if data.erro}
    <p class="aviso erro" role="alert">{data.erro}</p>
  {/if}

  <header class="cabecalho">
    <h1>Métricas do sistema</h1>
    <p class="sub">Usuários, correções e fila de execução.</p>
  </header>

  {#if metricas}
    {@const falhas = metricas.falhas_ultimos_7_dias}
    <div class="destaques">
      <div class="destaque">
        <span class="rotulo">Turmas ativas</span>
        <strong class="valor">{metricas.total_turmas_ativas}</strong>
      </div>
      <div class="destaque" class:falha={falhas > 0}>
        <span class="rotulo">Falhas nos últimos 7 dias</span>
        <strong class="valor">{falhas}</strong>
        <span class="nota">{falhas > 0 ? "Há correções que não terminaram." : "Nenhuma falha registrada."}</span>
      </div>
    </div>

    <div class="paineis">
      {#each paineis as painel (painel.titulo)}
        {@const total = painel.linhas.reduce((t, l) => t + l.qtd, 0)}
        <section class="painel">
          <h2>{painel.titulo}</h2>
          {#if painel.linhas.length}
            <ul class="metricas">
              {#each painel.linhas as l (l.chave)}
                <li>
                  <span class="linha"><span>{l.rotulo}</span><strong>{l.qtd}</strong></span>
                  <span class="barra" aria-hidden="true"><span style:width="{total ? (l.qtd / total) * 100 : 0}%"></span></span>
                </li>
              {/each}
            </ul>
          {:else}
            <p class="vazio">{painel.vazio}</p>
          {/if}
        </section>
      {/each}
    </div>
  {/if}
</section>

<style>
  .admin-dashboard {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .cabecalho h1 {
    margin: 0;
  }
  .sub {
    margin: 0.2rem 0 0;
    color: var(--seno-gray-500);
  }
  .destaques {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
    gap: 1rem;
  }
  .destaque {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    padding: 1.1rem 1.25rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
  }
  .destaque.falha {
    background: var(--seno-red-50);
    border-color: var(--seno-red-200);
  }
  .rotulo {
    color: var(--seno-gray-700);
    font-size: 0.9rem;
    font-weight: 500;
  }
  .valor {
    font-size: 2.4rem;
    line-height: 1.1;
    font-weight: 600;
    color: var(--seno-blue-900);
  }
  .falha .valor {
    color: var(--seno-red-700);
  }
  .nota {
    font-size: 0.85rem;
    color: var(--seno-gray-500);
  }
  .falha .nota {
    color: var(--seno-red-700);
  }
  .paineis {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(17rem, 1fr));
    gap: 1rem;
  }
  .painel {
    padding: 1.1rem 1.25rem 1.25rem;
    background: var(--seno-white);
    border: 1px solid var(--seno-line);
    border-radius: var(--raio-lg);
  }
  .painel h2 {
    margin: 0 0 0.9rem;
    font-size: 1rem;
  }
  .metricas {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .linha {
    display: flex;
    justify-content: space-between;
    font-size: 0.92rem;
    margin-bottom: 0.3rem;
  }
  .linha strong {
    color: var(--seno-blue-900);
  }
  .barra {
    display: block;
    height: 0.4rem;
    border-radius: var(--raio-pill);
    background: var(--seno-blue-100);
    overflow: hidden;
  }
  .barra > span {
    display: block;
    height: 100%;
    background: var(--seno-blue-600);
    border-radius: var(--raio-pill);
  }
  .painel .vazio {
    padding: 0.5rem 0;
    text-align: left;
  }
</style>
