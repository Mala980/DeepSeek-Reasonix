// The repository owner is never thanked on release pages; change the login here only.
export const repositoryOwner = "esengine";

export function withoutOwner(logins) {
  const owner = repositoryOwner.toLowerCase();
  return logins.filter((login) => String(login).toLowerCase() !== owner);
}
