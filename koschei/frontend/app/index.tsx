import { Link } from 'expo-router';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

const CYAN = '#00e5ff';
const VIOLET = '#9d4edd';
const MAGENTA = '#ff2bd1';
const GREEN = '#00ff9d';
const BG = '#03040a';

type ModuleItem = {
  ico: string;
  name: string;
  desc: string;
  accent: string;
  route: string;
};

const MODULES: ModuleItem[] = [
  { ico: '⌬', name: 'CODE ENGINE', desc: 'Üretim hazır kod akışı', accent: CYAN, route: '/m-code' },
  { ico: '◈', name: 'IMAGE FORGE', desc: 'Sinematik görsel üretimi', accent: MAGENTA, route: '/m-image' },
  { ico: '▣', name: 'VIDEO LAB', desc: 'Video ve sahne üretimi', accent: VIOLET, route: '/m-video' },
  { ico: '⬢', name: 'CHAT NEXUS', desc: 'Akıllı sohbet merkezi', accent: CYAN, route: '/m-chat' },
  { ico: '⟁', name: 'REASON MATRIX', desc: 'Adım adım düşünce ve karar analizi', accent: VIOLET, route: '/m-reason' },
  { ico: '✦', name: 'RESEARCH SCOUT', desc: 'Hızlı pazar/ürün araştırma özeti', accent: GREEN, route: '/m-research' },
  { ico: '◎', name: 'PROMPT LAB', desc: 'Prompt iyileştirme ve varyasyon', accent: CYAN, route: '/m-promptlab' },
];

function ModuleCard({ module }: { module: ModuleItem }) {
  const active = module.route !== '';
  const inner = (
    <View style={[styles.card, { borderColor: module.accent + '55' }]}>
      <View style={styles.cardTop}>
        <Text style={[styles.cardIco, { color: module.accent }]}>{module.ico}</Text>
        <View style={[styles.badge, { borderColor: active ? GREEN + '77' : '#ffffff22' }]}>
          <Text style={[styles.badgeText, { color: active ? GREEN : '#7f93a8' }]}>
            {active ? 'ACTIVE' : 'YAKINDA'}
          </Text>
        </View>
      </View>
      <Text style={styles.cardName}>{module.name}</Text>
      <Text style={styles.cardDesc}>{module.desc}</Text>
    </View>
  );

  if (active) {
    return (
      <Link href={module.route} asChild>
        <Pressable style={styles.cardWrap}>{inner}</Pressable>
      </Link>
    );
  }
  return <View style={styles.cardWrap}>{inner}</View>;
}

export default function Home() {
  return (
    <View style={styles.root}>
      <View style={styles.glowCyan} />
      <View style={styles.glowViolet} />

      <ScrollView contentContainerStyle={styles.scroll} showsVerticalScrollIndicator={false}>
        <View style={styles.statusBar}>
          <View style={styles.statusLeft}>
            <View style={styles.dot} />
            <Text style={styles.statusText}>SYSTEM ONLINE</Text>
          </View>
          <Text style={styles.statusText}>NODE: EU-CENTRAL-1</Text>
        </View>

        <Text style={styles.kicker}>// AI ÜRETİM MERKEZİ</Text>
        <Text style={styles.title}>KOSCHEI</Text>
        <Text style={styles.subtitle}>THE IMMORTAL AI</Text>

        <Text style={styles.tagline}>
          Tek komuta merkezi. Fikrini yaz — Koschei senin için üretsin.
        </Text>

        <Text style={styles.authLabel}>Hızlı Erişim</Text>
        <View style={styles.btns}>
          <Link href="/register" asChild>
            <Pressable style={[styles.btn, styles.btnPrimary]}>
              <Text style={styles.btnPrimaryText}>▶ SİSTEME GİR — ÜCRETSİZ</Text>
            </Pressable>
          </Link>
          <Link href="/login" asChild>
            <Pressable style={[styles.btn, styles.btnGhost]}>
              <Text style={styles.btnGhostText}>⬡ LOGIN</Text>
            </Pressable>
          </Link>
        </View>

        <View style={styles.sectionHead}>
          <View style={styles.sectionLine} />
          <Text style={styles.sectionText}>MODÜLLER</Text>
        </View>

        <View style={styles.grid}>
          {MODULES.map((m) => (
            <ModuleCard key={m.name} module={m} />
          ))}
        </View>

        <Text style={styles.footer}>KOSCHEI RUNTIME — TRADEPIGLOBALL.CO</Text>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: BG },

  glowCyan: {
    position: 'absolute',
    top: -120,
    left: -100,
    width: 280,
    height: 280,
    borderRadius: 140,
    backgroundColor: 'rgba(0,229,255,0.10)',
  },
  glowViolet: {
    position: 'absolute',
    bottom: -140,
    right: -110,
    width: 300,
    height: 300,
    borderRadius: 150,
    backgroundColor: 'rgba(157,78,221,0.10)',
  },

  scroll: { padding: 22, paddingTop: 56, paddingBottom: 60, maxWidth: 560, width: '100%', alignSelf: 'center' },

  statusBar: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderWidth: 1,
    borderColor: 'rgba(0,229,255,0.25)',
    backgroundColor: 'rgba(0,229,255,0.04)',
    borderRadius: 8,
    paddingVertical: 9,
    paddingHorizontal: 13,
    marginBottom: 34,
  },
  statusLeft: { flexDirection: 'row', alignItems: 'center' },
  dot: {
    width: 7,
    height: 7,
    borderRadius: 4,
    backgroundColor: GREEN,
    marginRight: 7,
  },
  statusText: { color: CYAN, fontSize: 10, letterSpacing: 2, fontFamily: 'monospace' },

  kicker: { color: CYAN, fontSize: 12, letterSpacing: 3, fontFamily: 'monospace' },
  title: {
    color: '#ffffff',
    fontSize: 58,
    fontWeight: '900',
    letterSpacing: 3,
    marginTop: 6,
  },
  subtitle: { color: VIOLET, fontSize: 13, letterSpacing: 4, marginTop: 2, fontFamily: 'monospace' },
  tagline: {
    color: '#8aa6bf',
    fontSize: 14,
    lineHeight: 22,
    marginTop: 20,
    marginBottom: 30,
  },

  authLabel: { color: '#7bb6d9', fontSize: 12, marginBottom: 8, letterSpacing: 1.2 },
  btns: { gap: 13, marginBottom: 40 },
  btn: { paddingVertical: 17, borderRadius: 11, alignItems: 'center' },
  btnPrimary: { backgroundColor: CYAN },
  btnPrimaryText: { color: '#00160f', fontSize: 14, fontWeight: '800', letterSpacing: 1.5 },
  btnGhost: {
    borderWidth: 1,
    borderColor: 'rgba(0,229,255,0.4)',
    backgroundColor: 'rgba(0,229,255,0.05)',
  },
  btnGhostText: { color: CYAN, fontSize: 14, fontWeight: '700', letterSpacing: 1.5 },

  sectionHead: { flexDirection: 'row', alignItems: 'center', gap: 9, marginBottom: 16 },
  sectionLine: { width: 24, height: 1, backgroundColor: VIOLET },
  sectionText: { color: VIOLET, fontSize: 11, letterSpacing: 3, fontFamily: 'monospace' },

  grid: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between' },
  cardWrap: { width: '48%', marginBottom: 13 },
  card: {
    borderWidth: 1,
    borderRadius: 12,
    padding: 15,
    backgroundColor: 'rgba(8,16,30,0.7)',
    minHeight: 124,
  },
  cardTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start' },
  cardIco: { fontSize: 24 },
  badge: {
    borderWidth: 1,
    borderRadius: 5,
    paddingHorizontal: 7,
    paddingVertical: 2,
  },
  badgeText: { fontSize: 8, letterSpacing: 1.5, fontWeight: '700' },
  cardName: { color: '#ffffff', fontSize: 14, fontWeight: '800', letterSpacing: 0.5, marginTop: 12 },
  cardDesc: { color: '#6f8aa3', fontSize: 10.5, marginTop: 4, lineHeight: 15 },

  footer: {
    color: '#3d5871',
    fontSize: 9,
    letterSpacing: 2,
    textAlign: 'center',
    marginTop: 36,
    fontFamily: 'monospace',
  },
});
