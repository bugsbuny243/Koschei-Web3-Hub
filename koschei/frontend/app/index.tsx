import { Link } from 'expo-router';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

const CYAN = '#00e5ff';
const VIOLET = '#9d4edd';
const MAGENTA = '#ff2bd1';
const GREEN = '#00ff9d';
const BG = '#03040a';

type ModuleStatus = 'ACTIVE' | 'LAB' | 'NEXT' | 'PAUSED' | 'ENTERPRISE FUTURE';

type ModuleItem = {
  ico: string;
  name: string;
  desc: string;
  accent: string;
  route?: string;
  status: ModuleStatus;
};

const MODULES: ModuleItem[] = [
  { ico: '⚙', name: 'Runtime Factory', desc: 'Agentic project planning and production contracts.', accent: CYAN, route: '/dashboard', status: 'ACTIVE' },
  { ico: '◈', name: 'Artifact Forge', desc: 'Generate downloadable delivery/code packages.', accent: VIOLET, route: '/dashboard', status: 'ACTIVE' },
  { ico: '⬢', name: 'AI Console', desc: 'Chat, code, and reasoning cockpit.', accent: CYAN, route: '/dashboard', status: 'ACTIVE' },
  { ico: '₿', name: 'Public SaaS Plans', desc: 'Shopier plans, credits, and manual activation.', accent: GREEN, route: '/pricing', status: 'ACTIVE' },
  { ico: '◎', name: 'UI Lab', desc: 'Koschei Command Universe prototype.', accent: MAGENTA, route: '/ui-lab', status: 'LAB' },
  { ico: '♛', name: 'Owner God Mode', desc: 'Internal client/Fiverr production cockpit.', accent: VIOLET, route: '/owner', status: 'NEXT' },
  { ico: '▣', name: 'Media Factory', desc: 'Image/audio/video modules paused to reduce cost.', accent: '#9ca3af', status: 'PAUSED' },
  { ico: '⟁', name: 'Cyber Defense', desc: 'Paused until company/team/legal foundation exists.', accent: '#9ca3af', route: '/cyber-defense', status: 'ENTERPRISE FUTURE' },
];

function ModuleCard({ module }: { module: ModuleItem }) {
  const active = Boolean(module.route);
  const inner = (
    <View style={[styles.card, { borderColor: module.accent + '55' }]}> 
      <View style={styles.cardTop}>
        <Text style={[styles.cardIco, { color: module.accent }]}>{module.ico}</Text>
        <View style={[styles.badge, { borderColor: active ? GREEN + '77' : '#ffffff22' }]}> 
          <Text style={[styles.badgeText, { color: active ? GREEN : '#7f93a8' }]}>{module.status}</Text>
        </View>
      </View>
      <Text style={styles.cardName}>{module.name}</Text>
      <Text style={styles.cardDesc}>{module.desc}</Text>
    </View>
  );
  return active ? <Link href={module.route as any} asChild><Pressable style={styles.cardWrap}>{inner}</Pressable></Link> : <View style={styles.cardWrap}>{inner}</View>;
}

export default function Home() {
  return (
    <View style={styles.root}>
      <View style={styles.glowCyan} />
      <View style={styles.glowViolet} />
      <ScrollView contentContainerStyle={styles.scroll} showsVerticalScrollIndicator={false}>
        <Text style={styles.kicker}>// KOSCHEI STRATEGIC CORE</Text>
        <Text style={styles.title}>KOSCHEI</Text>
        <Text style={styles.subtitle}>THE IMMORTAL AI</Text>
        <Text style={styles.tagline}>Focused command stack for Runtime Factory, Artifact Forge, and AI Console.</Text>

        <Text style={styles.authLabel}>Primary Access</Text>
        <View style={styles.btns}>
          <Link href="/dashboard" asChild><Pressable style={[styles.btn, styles.btnPrimary]}><Text style={styles.btnPrimaryText}>ENTER COMMAND CENTER</Text></Pressable></Link>
          <Link href="/pricing" asChild><Pressable style={[styles.btn, styles.btnGhost]}><Text style={styles.btnGhostText}>VIEW PLANS</Text></Pressable></Link>
          <Link href="/ui-lab" asChild><Pressable style={[styles.btn, styles.btnGhost]}><Text style={styles.btnGhostText}>UI LAB</Text></Pressable></Link>
        </View>

        <View style={styles.sectionHead}><View style={styles.sectionLine} /><Text style={styles.sectionText}>MODULE DIRECTION</Text></View>
        <View style={styles.grid}>{MODULES.map((m) => <ModuleCard key={m.name} module={m} />)}</View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: BG }, glowCyan: { position: 'absolute', top: -120, left: -100, width: 280, height: 280, borderRadius: 140, backgroundColor: 'rgba(0,229,255,0.10)' }, glowViolet: { position: 'absolute', bottom: -140, right: -110, width: 300, height: 300, borderRadius: 150, backgroundColor: 'rgba(157,78,221,0.10)' },
  scroll: { padding: 22, paddingTop: 56, paddingBottom: 60, maxWidth: 560, width: '100%', alignSelf: 'center' }, kicker: { color: CYAN, fontSize: 12, letterSpacing: 3, fontFamily: 'monospace' },
  title: { color: '#ffffff', fontSize: 58, fontWeight: '900', letterSpacing: 3, marginTop: 6 }, subtitle: { color: VIOLET, fontSize: 13, letterSpacing: 4, marginTop: 2, fontFamily: 'monospace' },
  tagline: { color: '#8aa6bf', fontSize: 14, lineHeight: 22, marginTop: 20, marginBottom: 20 }, authLabel: { color: '#7bb6d9', fontSize: 12, marginBottom: 8, letterSpacing: 1.2 },
  btns: { gap: 12, marginBottom: 28 }, btn: { paddingVertical: 15, borderRadius: 11, alignItems: 'center' }, btnPrimary: { backgroundColor: CYAN }, btnPrimaryText: { color: '#00160f', fontSize: 14, fontWeight: '800' },
  btnGhost: { borderWidth: 1, borderColor: 'rgba(0,229,255,0.4)', backgroundColor: 'rgba(0,229,255,0.05)' }, btnGhostText: { color: CYAN, fontSize: 14, fontWeight: '700' },
  sectionHead: { flexDirection: 'row', alignItems: 'center', gap: 9, marginBottom: 16 }, sectionLine: { width: 24, height: 1, backgroundColor: VIOLET }, sectionText: { color: VIOLET, fontSize: 11, letterSpacing: 3, fontFamily: 'monospace' },
  grid: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between' }, cardWrap: { width: '48%', marginBottom: 13 }, card: { borderWidth: 1, borderRadius: 12, padding: 15, backgroundColor: 'rgba(8,16,30,0.7)', minHeight: 124 },
  cardTop: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start' }, cardIco: { fontSize: 24 }, badge: { borderWidth: 1, borderRadius: 5, paddingHorizontal: 7, paddingVertical: 2 }, badgeText: { fontSize: 8, letterSpacing: 1.1, fontWeight: '700' }, cardName: { color: '#ffffff', fontSize: 14, fontWeight: '800', marginTop: 12 }, cardDesc: { color: '#6f8aa3', fontSize: 10.5, marginTop: 4, lineHeight: 15 },
});
