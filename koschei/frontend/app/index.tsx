import AsyncStorage from '@react-native-async-storage/async-storage';
import { useEffect, useMemo, useRef } from 'react';
import { Link, router } from 'expo-router';
import {
  Animated,
  Easing,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  useWindowDimensions,
  View,
} from 'react-native';

const GREEN = '#00ff9d';
const CYAN = '#00e5ff';
const VIOLET = '#9d4edd';
const MAGENTA = '#ff2bd1';
const AMBER = '#ffc857';
const GRAY = '#8aa0b8';
const BG = '#03040a';
const PANEL = 'rgba(5, 12, 25, 0.78)';

const MODULES = [
  { ico: '⌬', name: 'CODE ENGINE', desc: 'Üretim hazır kod akışı', accent: CYAN, route: '' },
  { ico: '◈', name: 'IMAGE FORGE', desc: 'Sinematik görsel üretimi', accent: MAGENTA, route: '/m-image' },
  { ico: '▣', name: 'VIDEO LAB', desc: 'Video fikir ve sahne üretimi', accent: VIOLET, route: '' },
  { ico: '◊', name: 'AUDIO CORE', desc: 'Seslendirme ve audio akışı', accent: GREEN, route: '' },
  { ico: '⬢', name: 'CHAT NEXUS', desc: 'Akıllı sohbet merkezi', accent: CYAN, route: '' },
  { ico: '⟁', name: 'REASON MATRIX', desc: 'Derin analiz ve karar motoru', accent: VIOLET, route: '' },
];

function RainColumn({ left, delay, duration }: { left: number; delay: number; duration: number }) {
  const y = useRef(new Animated.Value(-260)).current;
  useEffect(() => {
    const loop = Animated.loop(Animated.timing(y, { toValue: 980, duration, delay, easing: Easing.linear, useNativeDriver: true }));
    loop.start();
    return () => loop.stop();
  }, [delay, duration, y]);
  const chars = '01KOSCHEIΦΨλΔカキクケコ'.split('');
  return <Animated.View style={[styles.rainCol, { left, transform: [{ translateY: y }] }]}>{Array.from({ length: 18 }).map((_, i) => <Text key={i} style={[styles.rainText, { color: i === 0 ? 'rgba(0,255,157,0.9)' : i < 4 ? 'rgba(0,229,255,0.62)' : 'rgba(157,78,221,0.18)' }]}>{chars[(i + left) % chars.length]}</Text>)}</Animated.View>;
}

function GlowButton({ label, href, primary, onPress }: { label: string; href?: string; primary?: boolean; onPress?: () => void }) {
  const glow = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    if (!primary) return;
    const loop = Animated.loop(Animated.sequence([Animated.timing(glow, { toValue: 1, duration: 1300, useNativeDriver: false }), Animated.timing(glow, { toValue: 0, duration: 1300, useNativeDriver: false })]));
    loop.start();
    return () => loop.stop();
  }, [glow, primary]);
  const shadowRadius = glow.interpolate({ inputRange: [0, 1], outputRange: [10, 28] });
  const body = <Pressable onPress={onPress} style={({ pressed }) => pressed ? { opacity: 0.88 } : null}><Animated.View style={[styles.button, primary ? styles.buttonPrimary : styles.buttonGhost, primary ? { shadowRadius: shadowRadius as any } : null]}><Text style={[styles.buttonText, { color: primary ? '#00160f' : CYAN }]}>{label}</Text></Animated.View></Pressable>;
  return href ? <Link href={href} asChild>{body}</Link> : body;
}

function Ring({ size, color, duration, reverse }: { size: number; color: string; duration: number; reverse?: boolean }) {
  const spin = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    const loop = Animated.loop(Animated.timing(spin, { toValue: 1, duration, easing: Easing.linear, useNativeDriver: true }));
    loop.start();
    return () => loop.stop();
  }, [duration, spin]);
  const rotate = spin.interpolate({ inputRange: [0, 1], outputRange: reverse ? ['360deg', '0deg'] : ['0deg', '360deg'] });
  return <Animated.View style={[styles.ring, { width: size, height: size, borderRadius: size / 2, borderTopColor: color, borderRightColor: 'rgba(255,255,255,0.04)', borderBottomColor: 'rgba(0,229,255,0.12)', borderLeftColor: 'rgba(157,78,221,0.14)', transform: [{ rotate }] }]} />;
}

function QuantumCore() { /* unchanged visuals */
  const pulse = useRef(new Animated.Value(0)).current;
  useEffect(() => { const loop = Animated.loop(Animated.sequence([Animated.timing(pulse, { toValue: 1, duration: 1100, useNativeDriver: true }), Animated.timing(pulse, { toValue: 0, duration: 1100, useNativeDriver: true })])); loop.start(); return () => loop.stop(); }, [pulse]);
  const scale = pulse.interpolate({ inputRange: [0, 1], outputRange: [1, 1.1] });
  return <View style={styles.coreWrap}><View style={styles.coreHalo} /><Ring size={246} color={CYAN} duration={7200} /><Ring size={198} color={VIOLET} duration={5200} reverse /><Ring size={148} color={MAGENTA} duration={3900} /><Ring size={108} color={GREEN} duration={2800} reverse /><Animated.View style={[styles.core, { transform: [{ scale }] }]}><Text style={styles.coreTop}>AI CORE</Text><Text style={styles.coreNum}>99.9%</Text><Text style={styles.coreBottom}>ONLINE</Text></Animated.View></View>;
}

const getStatusStyle = (status: ModuleStatus) => status === 'ACTIVE' ? styles.badgeActive : status === 'NEXT' ? styles.badgeNext : status === 'PAUSED' ? styles.badgePaused : styles.badgeFuture;

export default function Home() {
  const { width } = useWindowDimensions();
  const rainColumns = useMemo(() => Array.from({ length: Math.min(18, Math.max(10, Math.floor(width / 34))) }).map((_, i) => ({ left: i * 38 + 4, delay: i * 220, duration: 5400 + (i % 5) * 850 })), [width]);

  const enterCommandCenter = async () => {
    const token = await AsyncStorage.getItem(TOKEN_KEY);
    router.push(token ? '/dashboard' : '/login');
  };

  return (
    <View style={styles.root}>
      <View style={styles.glowCyan} />
      <View style={styles.glowPurple} />
      <View style={styles.glowGreen} />
      <View style={styles.gridLayer} />

      <View style={styles.rainLayer} pointerEvents="none">
        {rainColumns.map((col, i) => (
          <RainColumn key={i} left={col.left} delay={col.delay} duration={col.duration} />
        ))}
      </View>

      <ScrollView contentContainerStyle={styles.scroll} showsVerticalScrollIndicator={false}>
        <View style={styles.topBar}>
          <View style={styles.brandBox}>
            <Text style={styles.brandMark}>K</Text>

            <View>
              <Text style={styles.brandName}>KOSCHEI</Text>
              <Text style={styles.brandSub}>IMMORTAL AI</Text>
            </View>
          </View>

          <View style={styles.statusPill}>
            <View style={styles.statusDot} />
            <Text style={styles.statusText}>ONLINE</Text>
          </View>
        </View>

        <View style={styles.heroPanel}>
          <View style={styles.statusRow}>
            <Text style={styles.microText}>● SYSTEM ONLINE</Text>
            <Text style={styles.microText}>NODE: EU-CENTRAL-1</Text>
          </View>

          <Text style={styles.kicker}>// SKYNET MATRIX QUANTUM RUNTIME</Text>
          <Text style={styles.title}>KOSCHEI</Text>
          <Text style={styles.titleSub}>THE IMMORTAL AI</Text>

          <Text style={styles.heroText}>
            Tek komutla kod, görsel, video, ses ve chat üret. Koschei tüm AI
            modellerini tek komuta merkezinde toplar.
          </Text>

          <QuantumCore />

          <View style={styles.buttonStack}>
            <GlowButton label="▶ SİSTEME GİR — ÜCRETSİZ" href="/register" primary />
            <GlowButton label="⬡ LOGIN" href="/login" />
            <GlowButton label="✦ ENTER UI LAB" href="/ui-lab" />
          </View>
        </View>

        <SectionTitle title="AKTİF MODÜLLER" />

        <View style={styles.moduleGrid}>
          {MODULES.map((module) => (
            module.route ? (
              <Link key={module.name} href={module.route} asChild>
                <Pressable
                  style={[
                    styles.moduleCard,
                    {
                      borderColor: `${module.accent}55`,
                      shadowColor: module.accent,
                    },
                  ]}
                >
                  <View style={styles.moduleTop}>
                    <Text style={[styles.moduleIcon, { color: module.accent }]}>{module.ico}</Text>

                    <View style={styles.activeBadge}>
                      <Text style={styles.activeText}>ACTIVE</Text>
                    </View>
                  </View>

                  <Text style={styles.moduleName}>{module.name}</Text>
                  <Text style={styles.moduleDesc}>{module.desc}</Text>
                </Pressable>
              </Link>
            ) : (
              <View
                key={module.name}
                style={[
                  styles.moduleCard,
                  {
                    borderColor: `${module.accent}55`,
                    shadowColor: module.accent,
                  },
                ]}
              >
                <View style={styles.moduleTop}>
                  <Text style={[styles.moduleIcon, { color: module.accent }]}>{module.ico}</Text>

                  <View style={styles.activeBadge}>
                    <Text style={styles.activeText}>YAKINDA</Text>
                  </View>
                </View>

                <Text style={styles.moduleName}>{module.name}</Text>
                <Text style={styles.moduleDesc}>{module.desc}</Text>
              </View>
            )
          ))}
        </View>

        <View style={styles.footerPanel}>
          <Text style={styles.footerTitle}>KOSCHEI RUNTIME v1.0</Text>
          <Text style={styles.footerText}>TRADEPIGLOBALL.CO // QUANTUM COMMAND CENTER</Text>
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({root:{flex:1,backgroundColor:BG,overflow:'hidden'},glowCyan:{position:'absolute',top:-90,left:-110,width:280,height:280,borderRadius:140,backgroundColor:'rgba(0,229,255,0.16)'},glowPurple:{position:'absolute',top:190,right:-160,width:340,height:340,borderRadius:170,backgroundColor:'rgba(157,78,221,0.17)'},glowGreen:{position:'absolute',bottom:120,left:-150,width:320,height:320,borderRadius:160,backgroundColor:'rgba(0,255,157,0.09)'},gridLayer:{position:'absolute',top:0,right:0,bottom:0,left:0,borderWidth:1,borderColor:'rgba(0,229,255,0.04)'},rainLayer:{position:'absolute',top:0,right:0,bottom:0,left:0,opacity:0.3},rainCol:{position:'absolute',top:0},rainText:{fontSize:12,lineHeight:16,fontFamily:'monospace'},scroll:{padding:16,paddingTop:34,paddingBottom:54},heroPanel:{borderWidth:1,borderColor:'rgba(0,229,255,0.22)',borderRadius:26,backgroundColor:PANEL,padding:18,marginBottom:24},title:{color:'#fff',fontSize:56,fontWeight:'900'},heroText:{color:'#b7c8dd',fontSize:14,lineHeight:22,marginTop:10},coreWrap:{height:260,alignItems:'center',justifyContent:'center'},coreHalo:{position:'absolute',width:190,height:190,borderRadius:95,backgroundColor:'rgba(0,229,255,0.12)'},ring:{position:'absolute',borderWidth:1.4},core:{width:112,height:112,borderRadius:56,borderWidth:1,borderColor:'rgba(0,255,157,0.65)',backgroundColor:'rgba(0,20,26,0.92)',alignItems:'center',justifyContent:'center'},coreTop:{color:CYAN,fontSize:9,fontWeight:'800'},coreNum:{color:'#fff',fontSize:28,fontWeight:'900'},coreBottom:{color:GREEN,fontSize:9,fontWeight:'900'},buttonStack:{marginTop:8},button:{minHeight:54,borderRadius:16,alignItems:'center',justifyContent:'center',marginBottom:12},buttonPrimary:{backgroundColor:GREEN},buttonGhost:{borderWidth:1,borderColor:'rgba(0,229,255,0.42)',backgroundColor:'rgba(0,229,255,0.055)'},buttonText:{fontSize:14,fontWeight:'900',letterSpacing:1.2},moduleGrid:{flexDirection:'row',flexWrap:'wrap',justifyContent:'space-between',marginBottom:24},moduleCard:{width:'48.6%',minHeight:170,borderWidth:1,borderRadius:18,backgroundColor:'rgba(5,12,25,0.82)',padding:14,marginBottom:12},moduleTop:{flexDirection:'row',justifyContent:'space-between',alignItems:'center'},moduleIcon:{fontSize:24,fontWeight:'900'},statusBadge:{borderWidth:1,borderRadius:999,paddingHorizontal:7,paddingVertical:3},badgeActive:{borderColor:'rgba(0,255,157,0.28)',backgroundColor:'rgba(0,255,157,0.08)'},badgeNext:{borderColor:'rgba(255,200,87,0.5)',backgroundColor:'rgba(157,78,221,0.22)'},badgePaused:{borderColor:'rgba(138,160,184,0.4)',backgroundColor:'rgba(138,160,184,0.1)'},badgeFuture:{borderColor:'rgba(255,43,209,0.45)',backgroundColor:'rgba(157,78,221,0.2)'},statusBadgeText:{color:'#d8f2ff',fontSize:7,fontWeight:'900',letterSpacing:0.9},moduleName:{color:'#fff',fontSize:13,fontWeight:'900',marginTop:16},moduleDesc:{color:'#8ea7c2',fontSize:11,lineHeight:16,marginTop:6},moduleNote:{color:CYAN,fontSize:10,fontWeight:'800',marginTop:12,letterSpacing:1},footerPanel:{borderTopWidth:1,borderColor:'rgba(0,229,255,0.12)',paddingTop:18,paddingBottom:18,alignItems:'center'},footerTitle:{color:'#fff',fontSize:12,letterSpacing:2,fontWeight:'900'},footerText:{color:'#52708e',fontSize:10,letterSpacing:1.2,marginTop:6,textAlign:'center'}});
