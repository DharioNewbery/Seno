// Tipos compartilhados do BFF: usuário e cargos espelham a API (/v1/auth).
export type Cargo = "student" | "professor" | "admin" | "super";

export interface Pessoa {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
}

export interface Usuario extends Pessoa {
  status: string;
  disabled_at?: string | null;
  roles: Cargo[];
  created_at: string;
  updated_at?: string | null;
}

export type PortalId = "aluno" | "professor" | "admin";

export const ROTULO_CARGO: Record<Cargo, string> = {
  student: "Aluno",
  professor: "Professor",
  admin: "Admin",
  super: "Super admin",
};

/** Portal associado a um cargo (admin e super compartilham o portal admin). */
export function portalPorCargo(cargo: Cargo): PortalId {
  switch (cargo) {
    case "professor":
      return "professor";
    case "admin":
    case "super":
      return "admin";
    default:
      return "aluno";
  }
}

/** Portais distintos que o usuário pode acessar, na ordem dos cargos. */
export function portaisDisponiveis(roles: Cargo[]): PortalId[] {
  const vistos = new Set<PortalId>();
  const lista: PortalId[] = [];
  for (const cargo of roles) {
    const portal = portalPorCargo(cargo);
    if (!vistos.has(portal)) {
      vistos.add(portal);
      lista.push(portal);
    }
  }
  return lista;
}

/** Cargos que dão acesso a um portal, para exibição no seletor. */
export function cargosDoPortal(roles: Cargo[], portal: PortalId): Cargo[] {
  return roles.filter((cargo) => portalPorCargo(cargo) === portal);
}
