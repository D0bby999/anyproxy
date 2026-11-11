// SPDX-License-Identifier: AGPL-3.0-or-later
// Proxy provider links, recommendation order, and logos adapted from AnyProfile (D0bby999/anyprofile).

export type ProviderType = 'residential' | 'isp' | 'mobile' | 'datacenter'

export type Provider = {
  id: string
  name: string
  url: string
  recommended?: boolean
  types: ProviderType[]
}

export const providerTypes: ProviderType[] = ['residential', 'isp', 'mobile', 'datacenter']

const all: ProviderType[] = ['residential', 'isp', 'mobile', 'datacenter']

export const providers: Provider[] = [
  { id: 'oxylabs', name: 'Oxylabs', url: 'https://oxylabs.go2cloud.org/aff_c?offer_id=7&aff_id=2794', recommended: true, types: all },
  { id: 'decodo', name: 'Decodo', url: 'https://dashboard.decodo.com/register?referral_code=2973fec4bd68c176365c5cf1ef80bade6eaccdde', types: all },
  { id: 'brightdata', name: 'Bright Data', url: 'https://get.brightdata.com/nhyeetzbk27n', types: all },
  { id: 'soax', name: 'SOAX', url: 'https://soax.com/', types: all },
  { id: 'evomi', name: 'Evomi', url: 'https://evomi.com/?p=5Z7DMFN3ME', types: all },
  { id: 'proxyempire', name: 'ProxyEmpire', url: 'https://proxyempire.io/?ref=oddhngm', types: all },
  { id: 'nodemaven', name: 'NodeMaven', url: 'https://nodemaven.com/?a_aid=anyprofile', types: ['residential', 'isp', 'mobile'] },
  { id: 'webshare', name: 'Webshare', url: 'https://www.webshare.io/?referral_code=e18t5gbij3d3', types: ['residential', 'isp', 'datacenter'] },
  { id: 'iproyal', name: 'IPRoyal', url: 'https://iproyal.sjv.io/X4aYqG', types: all },
  { id: 'proxyseller', name: 'Proxy-Seller', url: 'https://proxy-seller.com/?partner=JUJ9ZRI3NSJI1N', types: all },
  { id: 'massive', name: 'Massive', url: 'https://www.joinmassive.com/', types: ['residential', 'isp'] },
  { id: 'dataimpulse', name: 'DataImpulse', url: 'https://dataimpulse.com/?aff=e68ede41-33fe-4532-ab36-2240e6a1142d', types: ['residential', 'mobile', 'datacenter'] },
  { id: 'rayobyte', name: 'Rayobyte', url: 'https://rayobyte.com/products/residential-proxies/?ref=chau-d11bd8', types: all },
  { id: 'infatica', name: 'Infatica', url: 'https://dashboard.infatica.io/aff.php?aff=902', types: ['residential', 'isp', 'datacenter'] },
  { id: 'marsproxies', name: 'MarsProxies', url: 'https://marsproxies.sjv.io/bkJz7g', types: all },
  { id: 'proxycheap', name: 'Proxy-Cheap', url: 'https://app.proxy-cheap.com/r/NZNb2Ik0', types: all },
  { id: 'froxy', name: 'Froxy', url: 'https://froxy.com/', types: ['residential', 'mobile', 'datacenter'] },
  { id: 'coronium', name: 'Coronium', url: 'https://dashboard.coronium.io/en/sign-up#765b230d864e88ca7c0d800d4fd17c3b', types: ['mobile'] },
  { id: 'ltesocks', name: 'LTESocks', url: 'https://my.ltesocks.io/register?referrerToken=KSFSK9JQHEFK0KFL&referrer=01a17d5a-f6f2-7125-a17a-9dd8e2d583e3', types: ['mobile'] },
  { id: '9proxy', name: '9Proxy', url: 'https://9proxy.com/sign-up?inviteCode=9P_mXH5nLuQ', types: ['residential'] },
  { id: 'lumiproxy', name: 'LumiProxy', url: 'https://www.lumiproxy.com/?invite_code=97EJLV', types: ['residential', 'isp', 'datacenter'] },
  { id: 'rapidproxy', name: 'Rapidproxy', url: 'https://www.rapidproxy.io/?code=GN72DW2EP', types: ['residential', 'isp'] },
]

const logos = import.meta.glob<string>('../assets/partners/*.{png,svg}', { eager: true, import: 'default' })

/** Returns the bundled provider logo SVG or PNG URL if present. */
export function providerLogo(p: Provider): string | undefined {
  return logos[`../assets/partners/${p.id}.svg`] ?? logos[`../assets/partners/${p.id}.png`]
}
