1. **Criação do cliente:** `New` deve testar comunicação e consultar a versão do banco ou apenas validar a configuração?

2. **`IN` e `NOT IN`:** quais coleções aceitar e como tratar elementos `nil` dentro de listas não vazias?

3. **Paginação:** `Clone()` preserva a posição atual? Como navegar quando inserções ou exclusões mudarem a última página?

4. **Exclusividade das migrations:** o Krios bloqueia execuções concorrentes pelo banco ou a aplicação garante um único executor?

5. **Compatibilidade transacional:** como determinar se um script pode executar dentro de `sql.Tx`?

6. **Recuperação de migrations:** como reparar e retomar versões parcialmente aplicadas ou execuções com resultado incerto?

7. **Arquivos:** quando ler e validar os scripts, quais caminhos aceitar e como tratar arquivos ausentes?
