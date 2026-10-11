<script lang="ts">
  import { t } from "../lib/i18n.svelte";
  import { link } from "../lib/router.svelte";
</script>

<p class="text-md">{t("notFound.title")}</p>
<a href="/" use:link class="mt-2 inline-block text-indigo hover:underline">{t("notFound.back")}</a>
