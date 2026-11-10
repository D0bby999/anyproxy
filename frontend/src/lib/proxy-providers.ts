// SPDX-License-Identifier: AGPL-3.0-or-later
// Provider links and categories adapted from AnyProfile (D0bby999/anyprofile).
export type ProviderType = 'residential' | 'isp' | 'mobile' | 'datacenter'
export type Provider = { name: string; url: string; types: ProviderType[] }
const all: ProviderType[] = ['residential', 'isp', 'mobile', 'datacenter']
export const providers: Provider[] = [
  { name: 'Oxylabs', url: 'https://oxylabs.go2cloud.org/aff_c?offer_id=7&aff_id=2794', types: all },
  { name: 'Decodo', url: 'https://dashboard.decodo.com/register?referral_code=2973fec4bd68c176365c5cf1ef80bade6eaccdde', types: all },
  { name: 'Bright Data', url: 'https://get.brightdata.com/nhyeetzbk27n', types: all },
  { name: 'SOAX', url: 'https://soax.com/', types: all },
  { name: 'Evomi', url: 'https://evomi.com/?p=5Z7DMFN3ME', types: all },
  { name: 'ProxyEmpire', url: 'https://proxyempire.io/?ref=oddhngm', types: all },
  { name: 'NodeMaven', url: 'https://nodemaven.com/?a_aid=anyprofile', types: ['residential', 'isp', 'mobile'] },
  { name: 'Webshare', url: 'https://www.webshare.io/?referral_code=e18t5gbij3d3', types: ['residential', 'isp', 'datacenter'] },
  { name: 'IPRoyal', url: 'https://iproyal.sjv.io/X4aYqG', types: all },
  { name: 'Proxy-Seller', url: 'https://proxy-seller.com/?partner=JUJ9ZRI3NSJI1N', types: all },
  { name: 'Massive', url: 'https://www.joinmassive.com/', types: ['residential', 'isp'] },
  { name: 'DataImpulse', url: 'https://dataimpulse.com/?aff=e68ede41-33fe-4532-ab36-2240e6a1142d', types: ['residential', 'mobile', 'datacenter'] },
  { name: 'Rayobyte', url: 'https://rayobyte.com/products/residential-proxies/?ref=chau-d11bd8', types: all },
  { name: 'Infatica', url: 'https://dashboard.infatica.io/aff.php?aff=902', types: ['residential', 'isp', 'datacenter'] },
  { name: 'MarsProxies', url: 'https://marsproxies.sjv.io/bkJz7g', types: all },
  { name: 'Proxy-Cheap', url: 'https://app.proxy-cheap.com/r/NZNb2Ik0', types: all },
  { name: 'Froxy', url: 'https://froxy.com/', types: ['residential', 'mobile', 'datacenter'] },
  { name: 'Coronium', url: 'https://dashboard.coronium.io/en/sign-up#765b230d864e88ca7c0d800d4fd17c3b', types: ['mobile'] },
  { name: 'LTESocks', url: 'https://my.ltesocks.io/register?referrerToken=KSFSK9JQHEFK0KFL&referrer=01a17d5a-f6f2-7125-a17a-9dd8e2d583e3', types: ['mobile'] },
  { name: '9Proxy', url: 'https://9proxy.com/sign-up?inviteCode=9P_mXH5nLuQ', types: ['residential'] },
  { name: 'LumiProxy', url: 'https://www.lumiproxy.com/?invite_code=97EJLV', types: ['residential', 'isp', 'datacenter'] },
  { name: 'Rapidproxy', url: 'https://www.rapidproxy.io/?code=GN72DW2EP', types: ['residential', 'isp'] },
]
