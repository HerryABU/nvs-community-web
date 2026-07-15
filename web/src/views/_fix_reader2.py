c=open('Reader.vue','r',encoding='utf-8').read()

# Update checkSensitiveZone to also trigger for author custom wall
old='async function checkSensitiveZone() {\n  const cats = novelCategories.value.length > 0 ? novelCategories.value : (novelCategory.value ? [novelCategory.value] : []);\n  if (cats.length === 0) return;\n  const guard = await shouldShowGuard(cats, {'
new='async function checkSensitiveZone() {\n  // 作者自定义隔离墙：如果有 wall_warning 且 wall_enabled，直接触发\n  if (novelWallEnabled.value !== false && novelWallWarning.value) {\n    zoneGuardName.value = novelCategory.value || novelTitle.value || '该作品';\n    zoneGuardCross.value = false;\n    showZoneGuard.value = true;\n    return;\n  }\n  const cats = novelCategories.value.length > 0 ? novelCategories.value : (novelCategory.value ? [novelCategory.value] : []);\n  if (cats.length === 0) return;\n  const guard = await shouldShowGuard(cats, {'
if old in c: c=c.replace(old,new); print('OK')
open('Reader.vue','w',encoding='utf-8').write(c)
print('Done')
